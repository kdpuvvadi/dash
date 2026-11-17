package collector

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/puvvadi/dash/internal/store"
)

func CollectProxmox(run func(string) (string, error), hostRef string) ([]store.Guest, []store.Container, error) {
	if _, err := run("command -v pvesh"); err != nil {
		return nil, nil, nil
	}
	node := "localhost"
	if n, err := run("hostname -s"); err == nil && strings.TrimSpace(n) != "" {
		node = firstLine(n)
	}

	var guests []store.Guest
	qemuJSON, err := run(fmt.Sprintf("pvesh get /nodes/%s/qemu --output-format json", node))
	if err == nil {
		guests = append(guests, parseGuests(qemuJSON, "qemu")...)
	}
	lxcJSON, err := run(fmt.Sprintf("pvesh get /nodes/%s/lxc --output-format json", node))
	if err == nil {
		guests = append(guests, parseGuests(lxcJSON, "lxc")...)
	}

	// enrich with status/current when possible
	for i := range guests {
		g := &guests[i]
		path := fmt.Sprintf("pvesh get /nodes/%s/%s/%s/status/current --output-format json", node, map[string]string{"qemu": "qemu", "lxc": "lxc"}[g.Type], g.ID)
		out, err := run(path)
		if err != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(out), &m) != nil {
			continue
		}
		if v, ok := m["cpu"].(float64); ok {
			g.CPUPct = v * 100
		}
		if v, ok := m["mem"].(float64); ok {
			if max, ok2 := m["maxmem"].(float64); ok2 && max > 0 {
				g.MemPct = v / max * 100
				g.MaxMem = uint64(max)
			}
		}
		if v, ok := m["status"].(string); ok {
			g.Status = v
		}
	}

	var containers []store.Container
	for _, g := range guests {
		if g.Type != "lxc" || g.Status != "running" {
			continue
		}
		runIn := func(cmd string) (string, error) {
			return run(fmt.Sprintf("pct exec %s -- %s", g.ID, cmd))
		}
		cts, err := CollectDocker(runIn, hostRef, g.ID)
		if err != nil {
			continue
		}
		containers = append(containers, cts...)
	}
	return guests, containers, nil
}

func parseGuests(raw, typ string) []store.Guest {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil
	}
	var out []store.Guest
	for _, r := range rows {
		g := store.Guest{Type: typ, Status: "unknown"}
		switch v := r["vmid"].(type) {
		case float64:
			g.ID = strconv.Itoa(int(v))
		case string:
			g.ID = v
		}
		if n, ok := r["name"].(string); ok {
			g.Name = n
		}
		if s, ok := r["status"].(string); ok {
			g.Status = s
		}
		if g.Name == "" {
			g.Name = typ + "-" + g.ID
		}
		out = append(out, g)
	}
	return out
}
