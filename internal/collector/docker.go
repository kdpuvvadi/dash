package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/puvvadi/dash/internal/store"
)

const defaultDockerSock = "/var/run/docker.sock"

func CollectDocker(run func(string) (string, error), hostRef, guestID string) ([]store.Container, error) {
	return collectDocker(run, hostRef, guestID, false)
}

func CollectDockerLocal(run func(string) (string, error), hostRef, guestID string) ([]store.Container, error) {
	return collectDocker(run, hostRef, guestID, true)
}

func collectDocker(run func(string) (string, error), hostRef, guestID string, useLocalSock bool) ([]store.Container, error) {
	if useLocalSock {
		if sock := dockerSockPath(); sock != "" {
			if cts, err := CollectDockerSock(sock, hostRef, guestID); err == nil {
				return cts, nil
			}
		}
	}
	if _, err := run("command -v docker"); err != nil {
		return nil, nil
	}
	psOut, err := run(`docker ps -a --format '{"ID":"{{.ID}}","Names":"{{.Names}}","Image":"{{.Image}}","Status":"{{.Status}}","Ports":"{{.Ports}}"}'`)
	if err != nil {
		return nil, err
	}
	statsOut, _ := run(`docker stats --no-stream --format '{"ID":"{{.ID}}","CPUPerc":"{{.CPUPerc}}","MemUsage":"{{.MemUsage}}","NetIO":"{{.NetIO}}"}'`)

	statsByID := map[string]map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(statsOut))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]string
		if json.Unmarshal([]byte(line), &m) == nil {
			statsByID[m["ID"]] = m
		}
	}

	var out []store.Container
	sc = bufio.NewScanner(strings.NewReader(psOut))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row map[string]string
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		c := store.Container{
			ID:      row["ID"],
			Name:    row["Names"],
			Image:   row["Image"],
			Status:  row["Status"],
			HostRef: hostRef,
			GuestID: guestID,
			Ports:   parsePorts(row["Ports"]),
		}
		if st, ok := statsByID[c.ID]; ok {
			c.CPUPct = parsePct(st["CPUPerc"])
			c.MemUsage = st["MemUsage"]
			c.NetIO = st["NetIO"]
		}
		out = append(out, c)
	}
	return out, nil
}

func dockerSockPath() string {
	if v := strings.TrimSpace(os.Getenv("DOCKER_HOST")); strings.HasPrefix(v, "unix://") {
		p := strings.TrimPrefix(v, "unix://")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if st, err := os.Stat(defaultDockerSock); err == nil && !st.IsDir() {
		return defaultDockerSock
	}
	return ""
}

func CollectDockerSock(sock, hostRef, guestID string) ([]store.Container, error) {
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sock)
			},
		},
	}
	body, err := dockerGET(client, "/containers/json?all=true")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID      string   `json:"Id"`
		Names   []string `json:"Names"`
		Image   string   `json:"Image"`
		State   string   `json:"State"`
		Status  string   `json:"Status"`
		Ports   []struct {
			PublicPort int `json:"PublicPort"`
		} `json:"Ports"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}

	out := make([]store.Container, 0, len(rows))
	for _, row := range rows {
		name := ""
		if len(row.Names) > 0 {
			name = strings.TrimPrefix(row.Names[0], "/")
		}
		ports := make([]int, 0, len(row.Ports))
		seen := map[int]bool{}
		for _, p := range row.Ports {
			if p.PublicPort > 0 && !seen[p.PublicPort] {
				seen[p.PublicPort] = true
				ports = append(ports, p.PublicPort)
			}
		}
		c := store.Container{
			ID:      shortID(row.ID),
			Name:    name,
			Image:   row.Image,
			Status:  row.Status,
			HostRef: hostRef,
			GuestID: guestID,
			Ports:   ports,
		}
		if strings.EqualFold(row.State, "running") {
			if st, err := dockerContainerStats(client, row.ID); err == nil {
				c.CPUPct = st.cpu
				c.MemUsage = st.mem
				c.NetIO = st.net
			}
		}
		out = append(out, c)
	}
	return out, nil
}

type dockerStat struct {
	cpu float64
	mem string
	net string
}

func dockerContainerStats(client *http.Client, id string) (dockerStat, error) {
	body, err := dockerGET(client, "/containers/"+id+"/stats?stream=false")
	if err != nil {
		return dockerStat{}, err
	}
	var raw struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     uint32 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return dockerStat{}, err
	}
	var cpu float64
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage - raw.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(raw.CPUStats.SystemCPUUsage - raw.PreCPUStats.SystemCPUUsage)
	online := float64(raw.CPUStats.OnlineCPUs)
	if online == 0 {
		online = 1
	}
	if sysDelta > 0 && cpuDelta >= 0 {
		cpu = (cpuDelta / sysDelta) * online * 100
	}
	var rx, tx uint64
	for _, n := range raw.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	return dockerStat{
		cpu: cpu,
		mem: fmt.Sprintf("%s / %s", store.FormatBytes(raw.MemoryStats.Usage), store.FormatBytes(raw.MemoryStats.Limit)),
		net: fmt.Sprintf("%s / %s", store.FormatBytes(rx), store.FormatBytes(tx)),
	}, nil
}

func dockerGET(client *http.Client, path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker api %s: %s", path, resp.Status)
	}
	return body, nil
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func parsePct(s string) float64 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parsePorts(s string) []int {
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' '
	}) {
		if i := strings.Index(part, "->"); i >= 0 {
			left := part[:i]
			if j := strings.LastIndex(left, ":"); j >= 0 {
				p, err := strconv.Atoi(left[j+1:])
				if err == nil && !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		}
	}
	return out
}
