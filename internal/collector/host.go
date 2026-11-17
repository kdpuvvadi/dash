package collector

import (
	"bufio"
	"strconv"
	"strings"

	"github.com/puvvadi/dash/internal/store"
)

func CollectHostMetrics(run func(string) (string, error)) (*store.HostSnap, error) {
	h := &store.HostSnap{Online: true, Role: "linux", Extras: map[string]string{}}

	if uname, err := run("uname -n"); err == nil {
		h.Hostname = firstLine(uname)
	}
	if osrel, err := run("cat /etc/os-release"); err == nil {
		h.OS = parsePrettyName(osrel)
	}
	if load, err := run("cat /proc/loadavg"); err == nil {
		parts := strings.Fields(load)
		if len(parts) > 0 {
			h.Load1, _ = strconv.ParseFloat(parts[0], 64)
		}
	}
	if up, err := run("cut -d. -f1 /proc/uptime"); err == nil {
		h.UptimeSec, _ = strconv.ParseInt(strings.TrimSpace(up), 10, 64)
	}
	if mem, err := run("cat /proc/meminfo"); err == nil {
		h.MemTotal, h.MemUsed, h.MemPct = parseMeminfo(mem)
	}
	if df, err := run("df -B1 /"); err == nil {
		h.DiskTotal, h.DiskUsed, h.DiskPct = parseDF(df)
	}
	if cpu, err := run("grep 'cpu ' /proc/stat"); err == nil {
		h.CPUPct = parseCPUApprox(cpu)
	}
	if net, err := run("cat /proc/net/dev"); err == nil {
		h.NetRx, h.NetTx = parseNetDev(net)
	}

	if _, err := run("command -v pvesh"); err == nil {
		h.Role = "proxmox"
	} else if _, err := run("command -v docker"); err == nil {
		h.Role = "docker"
	}
	return h, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func parsePrettyName(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return "Linux"
}

func parseMeminfo(s string) (total, used uint64, pct float64) {
	var avail uint64
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(f[1], 10, 64)
		v *= 1024
		switch f[0] {
		case "MemTotal:":
			total = v
		case "MemAvailable:":
			avail = v
		}
	}
	if total > 0 {
		used = total - avail
		pct = float64(used) / float64(total) * 100
	}
	return
}

func parseDF(s string) (total, used uint64, pct float64) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 {
		return
	}
	f := strings.Fields(lines[len(lines)-1])
	if len(f) < 5 {
		return
	}
	total, _ = strconv.ParseUint(f[1], 10, 64)
	used, _ = strconv.ParseUint(f[2], 10, 64)
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return
}

func parseCPUApprox(line string) float64 {
	// idle-based instantaneous approx from single sample is weak; use load as proxy fallback later
	f := strings.Fields(line)
	if len(f) < 5 {
		return 0
	}
	var sum, idle float64
	for i := 1; i < len(f); i++ {
		v, _ := strconv.ParseFloat(f[i], 64)
		sum += v
		if i == 4 {
			idle = v
		}
	}
	if sum == 0 {
		return 0
	}
	return (1 - idle/sum) * 100
}

func parseNetDev(s string) (rx, tx uint64) {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
	}
	return
}
