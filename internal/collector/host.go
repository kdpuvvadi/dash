package collector

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/puvvadi/dash/internal/store"
)

func hostRoot() string {
	if v := strings.TrimSpace(os.Getenv("DASH_HOST_ROOT")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if st, err := os.Stat("/host/proc"); err == nil && st.IsDir() {
		return "/host"
	}
	return ""
}

func hostPath(p string) string {
	root := hostRoot()
	if root == "" {
		return p
	}
	return filepath.Join(root, strings.TrimPrefix(p, "/"))
}

// withHostMountNS wraps run() so /proc reads see the real machine (LXC/VM host)
// instead of the container's view. Docker-in-LXC bypasses lxcfs, so MemTotal looks
// like the Proxmox node; nsenter -t 1 -m restores the guest's virtualized /proc.
// Requires pid: host and CAP_SYS_ADMIN (see compose.yaml). Disable with DASH_NSENTER=0.
func withHostMountNS(run func(string) (string, error)) func(string) (string, error) {
	if !canNsenterHost() {
		return run
	}
	return func(cmd string) (string, error) {
		return run("nsenter -t 1 -m -- sh -c " + shellQuote(cmd))
	}
}

func canNsenterHost() bool {
	if v := strings.TrimSpace(os.Getenv("DASH_NSENTER")); v == "0" || strings.EqualFold(v, "false") {
		return false
	}
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return false
	}
	if _, err := exec.LookPath("nsenter"); err != nil {
		return false
	}
	out, err := exec.Command("nsenter", "-t", "1", "-m", "--", "true").CombinedOutput()
	return err == nil && len(strings.TrimSpace(string(out))) == 0
}

func CollectHostMetrics(run func(string) (string, error)) (*store.HostSnap, error) {
	h := &store.HostSnap{Online: true, Role: "linux", Extras: map[string]string{}}
	root := hostRoot()

	unameCmd := "uname -n"
	osCmd := "cat " + shellQuote(hostPath("/etc/os-release"))
	loadCmd := "cat " + shellQuote(hostPath("/proc/loadavg"))
	upCmd := "cut -d. -f1 " + shellQuote(hostPath("/proc/uptime"))
	memCmd := "cat " + shellQuote(hostPath("/proc/meminfo"))
	dfTarget := "/"
	if root != "" {
		dfTarget = root
	}
	dfCmd := "df -B1 " + shellQuote(dfTarget)
	cpuCmd := "grep 'cpu ' " + shellQuote(hostPath("/proc/stat"))
	netCmd := "cat " + shellQuote(hostPath("/proc/net/dev"))

	if uname, err := run(unameCmd); err == nil {
		h.Hostname = firstLine(uname)
	}
	if osrel, err := run(osCmd); err == nil {
		h.OS = parsePrettyName(osrel)
	}
	if load, err := run(loadCmd); err == nil {
		parts := strings.Fields(load)
		if len(parts) > 0 {
			h.Load1, _ = strconv.ParseFloat(parts[0], 64)
		}
	}
	if up, err := run(upCmd); err == nil {
		h.UptimeSec, _ = strconv.ParseInt(strings.TrimSpace(up), 10, 64)
	}
	if mem, err := run(memCmd); err == nil {
		h.MemTotal, h.MemUsed, h.MemPct = parseMeminfo(mem)
	}
	if df, err := run(dfCmd); err == nil {
		h.DiskTotal, h.DiskUsed, h.DiskPct = parseDF(df)
	}
	if cpu1, err := run(cpuCmd); err == nil {
		time.Sleep(200 * time.Millisecond)
		if cpu2, err2 := run(cpuCmd); err2 == nil {
			h.CPUPct = parseCPUDelta(cpu1, cpu2)
		} else {
			h.CPUPct = parseCPUApprox(cpu1)
		}
	}
	if net, err := run(netCmd); err == nil {
		h.NetRx, h.NetTx = parseNetDev(net)
	}

	if _, err := run("command -v pvesh"); err == nil {
		h.Role = "proxmox"
	} else if _, err := run("command -v docker"); err == nil {
		h.Role = "docker"
	}
	return h, nil
}

func shellQuote(p string) string {
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
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

func parseCPUDelta(a, b string) float64 {
	fa := strings.Fields(a)
	fb := strings.Fields(b)
	if len(fa) < 5 || len(fb) < 5 {
		return parseCPUApprox(b)
	}
	var sumA, idleA, sumB, idleB float64
	for i := 1; i < len(fa) && i < len(fb); i++ {
		va, _ := strconv.ParseFloat(fa[i], 64)
		vb, _ := strconv.ParseFloat(fb[i], 64)
		sumA += va
		sumB += vb
		if i == 4 {
			idleA, idleB = va, vb
		}
	}
	dTotal := sumB - sumA
	dIdle := idleB - idleA
	if dTotal <= 0 {
		return 0
	}
	pct := (1 - dIdle/dTotal) * 100
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
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
		if name == "lo" || strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "docker") {
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
