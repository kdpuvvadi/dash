package collector

import (
	"bufio"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/puvvadi/dash/internal/store"
)

func CollectDocker(run func(string) (string, error), hostRef, guestID string) ([]store.Container, error) {
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
		// look for host port patterns like 0.0.0.0:8123->8123/tcp or :::8123->
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
