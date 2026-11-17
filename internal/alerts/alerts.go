package alerts

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/puvvadi/dash/internal/config"
	"github.com/puvvadi/dash/internal/store"
)

type Checker struct {
	cfg    *config.Config
	client *http.Client
	last   map[string]time.Time
}

func New(cfg *config.Config) *Checker {
	return &Checker{
		cfg:    cfg,
		client: &http.Client{Timeout: 8 * time.Second},
		last:   map[string]time.Time{},
	}
}

func (c *Checker) SetConfig(cfg *config.Config) { c.cfg = cfg }

func (c *Checker) Evaluate(snap store.Snapshot) []store.AlertEvent {
	var events []store.AlertEvent
	now := time.Now().UTC()
	cfg := c.cfg
	for name, h := range snap.Hosts {
		if !h.Online {
			ev := store.AlertEvent{Level: "critical", Host: name, Message: name + " is offline", At: now}
			events = append(events, ev)
			c.notify(ev)
			continue
		}
		if h.DiskPct >= cfg.Alerts.DiskPercent {
			ev := store.AlertEvent{Level: "warning", Host: name, Message: name + " disk high", At: now}
			events = append(events, ev)
			c.notify(ev)
		}
		if h.MemPct >= cfg.Alerts.MemPercent {
			ev := store.AlertEvent{Level: "warning", Host: name, Message: name + " memory high", At: now}
			events = append(events, ev)
			c.notify(ev)
		}
	}
	// HTTP uptime for configured service URLs
	for _, h := range cfg.Hosts {
		for _, app := range h.ServiceRefs() {
			if app.URL == "" {
				continue
			}
			ok := checkURL(c.client, app.URL)
			if !ok {
				ev := store.AlertEvent{Level: "warning", Host: h.Name, Message: app.Type + " URL not reachable", At: now}
				events = append(events, ev)
				c.notify(ev)
			}
		}
	}
	return events
}

func checkURL(client *http.Client, raw string) bool {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func (c *Checker) notify(ev store.AlertEvent) {
	key := ev.Host + "|" + ev.Message
	if t, ok := c.last[key]; ok && time.Since(t) < 30*time.Minute {
		return
	}
	c.last[key] = time.Now()
	if c.cfg.Alerts.NtfyURL != "" {
		req, _ := http.NewRequest(http.MethodPost, c.cfg.Alerts.NtfyURL, bytes.NewBufferString(ev.Message))
		req.Header.Set("Title", "Dash alert")
		req.Header.Set("Priority", "default")
		_, _ = c.client.Do(req)
	}
	if c.cfg.Alerts.WebhookURL != "" {
		body, _ := json.Marshal(ev)
		_, _ = c.client.Post(c.cfg.Alerts.WebhookURL, "application/json", bytes.NewReader(body))
	}
}
