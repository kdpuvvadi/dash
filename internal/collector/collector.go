package collector

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/puvvadi/dash/internal/apps"
	"github.com/puvvadi/dash/internal/config"
	"github.com/puvvadi/dash/internal/store"
)

type Hub interface {
	Broadcast(snap store.Snapshot)
}

type netSample struct {
	rx, tx uint64
	at     time.Time
}

type Collector struct {
	cfg     *config.Config
	store   *store.Store
	defs    map[string]*apps.Def
	secrets config.Secrets
	hub     Hub
	mu      sync.Mutex
	prevNet map[string]netSample
}

func New(cfg *config.Config, st *store.Store, defs map[string]*apps.Def, secrets config.Secrets, hub Hub) *Collector {
	return &Collector{cfg: cfg, store: st, defs: defs, secrets: secrets, hub: hub, prevNet: map[string]netSample{}}
}

func (c *Collector) SetConfig(cfg *config.Config, defs map[string]*apps.Def, secrets config.Secrets) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
	c.defs = defs
	c.secrets = secrets
}

func (c *Collector) Run(ctx context.Context) {
	c.CollectOnce()
	ticker := time.NewTicker(time.Duration(c.cfg.PollSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.CollectOnce()
		}
	}
}

func (c *Collector) CollectOnce() {
	c.mu.Lock()
	cfg := c.cfg
	defs := c.defs
	secrets := c.secrets
	c.mu.Unlock()

	snap := store.Snapshot{
		Hosts: map[string]*store.HostSnap{},
		Apps:  map[string]*store.AppSnap{},
	}
	now := time.Now().UTC()
	var unknown []store.UnknownApp
	onlineHosts := 0

	for _, h := range cfg.Hosts {
		hs, unk := c.collectHost(h, defs)
		hs.Name = h.Name
		hs.Icon = h.Icon
		hs.Category = h.Category
		hs.Group = h.Group
		hs.Tags = h.Tags
		if hs.Icon == "" {
			switch hs.Role {
			case "proxmox":
				hs.Icon = "proxmox"
			case "docker":
				hs.Icon = "docker"
			default:
				hs.Icon = "linux"
			}
		}
		snap.Hosts[h.Name] = hs
		unknown = append(unknown, unk...)
		c.applyNetRates(h.Name, hs, now)
		if hs.Online {
			onlineHosts++
			_ = c.store.Record(h.Name, "cpu", hs.CPUPct, now)
			_ = c.store.Record(h.Name, "mem", hs.MemPct, now)
			_ = c.store.Record(h.Name, "disk", hs.DiskPct, now)
			_ = c.store.Record(h.Name, "net_rx", hs.NetRxBps, now)
			_ = c.store.Record(h.Name, "net_tx", hs.NetTxBps, now)
		}

		// Explicit apps in app.yaml (optional overrides)
		explicit := map[string]bool{}
		for _, app := range h.Apps {
			explicit[app.Type] = true
			def := defs[app.Type]
			key := h.Name + ":" + app.Type
			as := &store.AppSnap{
				ID:       key,
				Type:     app.Type,
				HostRef:  h.Name,
				URL:      app.URL,
				Category: app.Category,
				Group:    app.Group,
				Fields:   map[string]string{},
			}
			if as.Category == "" {
				as.Category = h.Category
			}
			if as.Group == "" {
				as.Group = h.Group
			}
			if def == nil {
				as.Error = "unknown app type — request a built-in or drop a custom def in apps.d"
				as.Name = app.Type
				snap.Apps[key] = as
				continue
			}
			as.Name = def.Name
			as.Icon = app.Icon
			if as.Icon == "" {
				as.Icon = def.UI.Icon
			}
			creds := mergeCreds(secrets.ForApp(app.Type), apps.Creds{
				"key": app.Key, "username": app.Username, "password": app.Password,
			})
			res := def.CollectMetrics(app.URL, creds)
			as.Online = res.Online
			as.NeedsKey = res.NeedsKey
			as.Fields = res.Fields
			as.Error = res.Error
			snap.Apps[key] = as
		}

		// App-managed: match containers to built-in (or custom) defs and collect automatically
		for _, ct := range hs.Containers {
			matched := false
			for _, def := range defs {
				if !def.Matches(ct.Image, ct.Name, ct.Ports) {
					continue
				}
				matched = true
				if explicit[def.ID] {
					break
				}
				url := guessAppURL(h, ct, def)
				key := h.Name + ":" + def.ID
				if _, exists := snap.Apps[key]; exists {
					key = h.Name + ":" + def.ID + ":" + ct.Name
				}
				as := &store.AppSnap{
					ID:       key,
					Type:     def.ID,
					Name:     def.Name,
					HostRef:  h.Name,
					URL:      url,
					Icon:     def.UI.Icon,
					Fields:   map[string]string{},
					Category: h.Category,
					Group:    h.Group,
				}
				creds := mergeCreds(secrets.ForApp(def.ID), nil)
				if url == "" {
					as.NeedsKey = len(def.Credentials) > 0
					as.Error = "found " + ct.Name + " — set URL in secrets or app.yaml if needed"
					snap.Apps[key] = as
					break
				}
				res := def.CollectMetrics(url, creds)
				as.Online = res.Online
				as.NeedsKey = res.NeedsKey
				as.Fields = res.Fields
				as.Error = res.Error
				if as.NeedsKey {
					as.Error = "needs API key — add it under " + def.ID + " in config/secrets.yaml"
				}
				snap.Apps[key] = as
				break
			}
			if !matched && strings.Contains(strings.ToLower(ct.Status), "up") {
				unknown = append(unknown, store.UnknownApp{
					HostRef: h.Name,
					Name:    ct.Name,
					Image:   ct.Image,
					Ports:   ct.Ports,
				})
			}
		}
	}

	snap.Unknown = dedupeUnknown(unknown)
	snap.Summary = buildSummary(snap, c.store.Started())
	_ = c.store.Prune(now.Add(-time.Duration(cfg.HistoryHours) * time.Hour))
	c.store.SetSnapshot(snap)
	if c.hub != nil {
		c.hub.Broadcast(snap)
	}
}

func (c *Collector) collectHost(h config.Host, defs map[string]*apps.Def) (*store.HostSnap, []store.UnknownApp) {
	_ = defs
	if h.Local {
		return c.collectLocal(h.Name)
	}
	client, err := DialSSH(h.Host, h.User, h.Port, h.IdentityFile)
	if err != nil {
		return &store.HostSnap{Online: false, Role: "linux", Error: err.Error()}, nil
	}
	defer client.Close()
	run := func(cmd string) (string, error) { return client.RunOK(cmd) }
	hs, err := CollectHostMetrics(run)
	if err != nil {
		return &store.HostSnap{Online: false, Error: err.Error()}, nil
	}
	if hs.Role == "proxmox" {
		guests, nested, err := CollectProxmox(run, h.Name)
		if err == nil {
			hs.Guests = guests
			hs.Containers = append(hs.Containers, nested...)
		}
	}
	cts, err := CollectDocker(run, h.Name, "")
	if err == nil {
		hs.Containers = append(hs.Containers, cts...)
	}
	if hs.CPUPct == 0 && hs.Load1 > 0 {
		hs.CPUPct = min(hs.Load1*25, 100)
	}
	return hs, nil
}

func (c *Collector) collectLocal(name string) (*store.HostSnap, []store.UnknownApp) {
	run := func(cmd string) (string, error) {
		out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	// On Windows dev hosts, local sh may fail — still try docker
	hs, err := CollectHostMetrics(run)
	if err != nil || hs == nil {
		hs = &store.HostSnap{Online: true, Role: "docker", Hostname: name}
	}
	cts, derr := CollectDocker(run, name, "")
	if derr != nil {
		// try docker without sh
		if out, err := exec.Command("docker", "ps", "-a", "--format", `{"ID":"{{.ID}}","Names":"{{.Names}}","Image":"{{.Image}}","Status":"{{.Status}}","Ports":"{{.Ports}}"}`).CombinedOutput(); err == nil {
			run2 := func(cmd string) (string, error) {
				if strings.HasPrefix(cmd, "docker ") {
					parts := strings.Fields(cmd)
					o, e := exec.Command(parts[0], parts[1:]...).CombinedOutput()
					return string(o), e
				}
				return "", fmt.Errorf("unsupported")
			}
			_ = out
			cts, _ = CollectDocker(run2, name, "")
		}
	} else {
		hs.Containers = cts
	}
	if len(hs.Containers) > 0 {
		hs.Online = true
		if hs.Role == "linux" {
			hs.Role = "docker"
		}
	}
	return hs, nil
}

func (c *Collector) applyNetRates(name string, hs *store.HostSnap, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev, ok := c.prevNet[name]
	c.prevNet[name] = netSample{rx: hs.NetRx, tx: hs.NetTx, at: now}
	if !ok || now.Before(prev.at) || hs.NetRx < prev.rx || hs.NetTx < prev.tx {
		return
	}
	sec := now.Sub(prev.at).Seconds()
	if sec <= 0 {
		return
	}
	hs.NetRxBps = float64(hs.NetRx-prev.rx) / sec
	hs.NetTxBps = float64(hs.NetTx-prev.tx) / sec
}

func buildSummary(snap store.Snapshot, started time.Time) store.Summary {
	total := 0
	online := 0
	var diskUsed, diskTotal, memUsed, memTotal uint64
	var uptime int64
	var cpuSum, rxBps, txBps float64
	var cpuN int
	for _, h := range snap.Hosts {
		total++
		if h.Online {
			online++
		}
		diskUsed += h.DiskUsed
		diskTotal += h.DiskTotal
		memUsed += h.MemUsed
		memTotal += h.MemTotal
		rxBps += h.NetRxBps
		txBps += h.NetTxBps
		if h.Online {
			cpuSum += h.CPUPct
			cpuN++
		}
		if h.UptimeSec > uptime {
			uptime = h.UptimeSec
		}
	}
	for _, a := range snap.Apps {
		total++
		if a.Online {
			online++
		}
	}
	status := "All systems operational"
	if online < total {
		status = "Some systems need attention"
	}
	if online == 0 && total > 0 {
		status = "No systems reachable"
	}
	sum := store.Summary{
		Status:         status,
		ServicesOnline: online,
		ServicesTotal:  total,
		UptimeHuman:    store.FormatUptime(uptime),
		MemUsed:        store.FormatBytes(memUsed),
		MemTotal:       store.FormatBytes(memTotal),
		StorageUsed:    store.FormatBytes(diskUsed),
		StorageTotal:   store.FormatBytes(diskTotal),
		NetRxBps:       rxBps,
		NetTxBps:       txBps,
		NetRxHuman:     store.FormatRate(rxBps),
		NetTxHuman:     store.FormatRate(txBps),
	}
	if cpuN > 0 {
		sum.CPUPct = cpuSum / float64(cpuN)
	}
	if memTotal > 0 {
		sum.MemPct = float64(memUsed) / float64(memTotal) * 100
	}
	if diskTotal > 0 {
		sum.StoragePct = float64(diskUsed) / float64(diskTotal) * 100
	}
	if uptime == 0 {
		sum.UptimeHuman = store.FormatUptime(int64(time.Since(started).Seconds()))
	}
	return sum
}

func dedupeUnknown(in []store.UnknownApp) []store.UnknownApp {
	seen := map[string]bool{}
	var out []store.UnknownApp
	for _, u := range in {
		k := u.HostRef + "|" + u.Name + "|" + u.Image
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, u)
	}
	return out
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mergeCreds(base map[string]string, overlay apps.Creds) apps.Creds {
	out := apps.Creds{}
	for k, v := range base {
		if v != "" {
			out[k] = v
		}
	}
	for k, v := range overlay {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func guessAppURL(h config.Host, ct store.Container, def *apps.Def) string {
	port := 0
	if len(ct.Ports) > 0 {
		port = ct.Ports[0]
	} else if len(def.Match.Ports) > 0 {
		port = def.Match.Ports[0]
	}
	if port == 0 {
		return ""
	}
	host := h.Host
	if h.Local {
		host = "127.0.0.1"
	}
	if host == "" {
		return ""
	}
	scheme := "http"
	if port == 443 || port == 8443 || port == 9443 {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}
