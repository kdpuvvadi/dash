package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

type Config struct {
	PollSeconds   int        `yaml:"poll_seconds"`
	HistoryHours  int        `yaml:"history_hours"`
	Listen        string     `yaml:"listen"`
	Alerts        Alerts     `yaml:"alerts"`
	Categories    []Category `yaml:"categories"`
	Groups        []Group    `yaml:"groups"`
	Layout        Layout     `yaml:"layout"`
	Hosts         []Host     `yaml:"hosts"`
	ConfigDir     string     `yaml:"-"`
	AppDefDir     string     `yaml:"-"`
	BundledAppDir string     `yaml:"-"`
}

type Alerts struct {
	NtfyURL      string  `yaml:"ntfy_url"`
	WebhookURL   string  `yaml:"webhook_url"`
	DiskPercent  float64 `yaml:"disk_percent"`
	MemPercent   float64 `yaml:"mem_percent"`
}

type Category struct {
	ID   string `yaml:"id" json:"id"`
	Name string `yaml:"name" json:"name"`
}

type Group struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Category string `yaml:"category" json:"category,omitempty"`
}

type Layout struct {
	Sections []Section `yaml:"sections" json:"sections"`
}

type Section struct {
	ID       string `yaml:"id" json:"id"`
	Title    string `yaml:"title" json:"title,omitempty"`
	Category string `yaml:"category" json:"category,omitempty"`
	Rows     []Row  `yaml:"rows" json:"rows"`
}

type Row struct {
	Columns int     `yaml:"columns" json:"columns"`
	Group   string  `yaml:"group" json:"group,omitempty"`
	Blocks  []Block `yaml:"blocks" json:"blocks"`
}

type Block struct {
	Type  string `yaml:"type" json:"type"`
	Ref   string `yaml:"ref" json:"ref,omitempty"`
	Field string `yaml:"field" json:"field,omitempty"`
	Icon  string `yaml:"icon" json:"icon,omitempty"`
}

type Host struct {
	Name         string   `yaml:"name"`
	Host         string   `yaml:"host"`
	User         string   `yaml:"user"`
	Port         int      `yaml:"port"`
	IdentityFile string   `yaml:"identity_file"`
	Local        bool     `yaml:"local"`
	Icon         string   `yaml:"icon"`
	Category     string   `yaml:"category"`
	Group        string   `yaml:"group"`
	Tags         []string `yaml:"tags"`
	Apps         []AppRef `yaml:"apps"`
}

type AppRef struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Key      string `yaml:"key"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Category string `yaml:"category"`
	Group    string `yaml:"group"`
	Icon     string `yaml:"icon"`
}

func DefaultConfigDir() string {
	if d := os.Getenv("DASH_CONFIG"); d != "" {
		return d
	}
	return "/config"
}

func FindAppFile(dir string) (string, error) {
	for _, name := range []string{"app.yaml", "app.yml"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no app.yaml or app.yml in %s", dir)
}

func Load(dir string) (*Config, error) {
	path, err := FindAppFile(dir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded := expandEnv(string(raw))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.applyDefaults()
	cfg.ConfigDir = dir
	cfg.AppDefDir = filepath.Join(dir, "apps.d")
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.PollSeconds <= 0 {
		c.PollSeconds = 5
	}
	if c.HistoryHours <= 0 {
		c.HistoryHours = 24
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.Alerts.DiskPercent == 0 {
		c.Alerts.DiskPercent = 90
	}
	if c.Alerts.MemPercent == 0 {
		c.Alerts.MemPercent = 95
	}
	for i := range c.Hosts {
		if c.Hosts[i].User == "" {
			c.Hosts[i].User = "root"
		}
		if c.Hosts[i].Port == 0 {
			c.Hosts[i].Port = 22
		}
	}
}

func (c *Config) Validate() error {
	catIDs := map[string]bool{}
	for _, cat := range c.Categories {
		if cat.ID == "" {
			return fmt.Errorf("category missing id")
		}
		catIDs[cat.ID] = true
	}
	groupIDs := map[string]bool{}
	for _, g := range c.Groups {
		if g.ID == "" {
			return fmt.Errorf("group missing id")
		}
		groupIDs[g.ID] = true
		if g.Category != "" && len(catIDs) > 0 && !catIDs[g.Category] {
			return fmt.Errorf("group %q references unknown category %q", g.ID, g.Category)
		}
	}
	hostNames := map[string]bool{}
	for _, h := range c.Hosts {
		if h.Name == "" {
			return fmt.Errorf("host missing name")
		}
		if !h.Local && h.Host == "" {
			return fmt.Errorf("host %q needs host or local: true", h.Name)
		}
		hostNames[h.Name] = true
		if h.Category != "" && len(catIDs) > 0 && !catIDs[h.Category] {
			return fmt.Errorf("host %q references unknown category %q", h.Name, h.Category)
		}
		if h.Group != "" && len(groupIDs) > 0 && !groupIDs[h.Group] {
			return fmt.Errorf("host %q references unknown group %q", h.Name, h.Group)
		}
	}
	for _, sec := range c.Layout.Sections {
		for _, row := range sec.Rows {
			if row.Columns <= 0 {
				return fmt.Errorf("layout row in section %q needs columns > 0", sec.ID)
			}
			if row.Group != "" && len(groupIDs) > 0 && !groupIDs[row.Group] {
				return fmt.Errorf("layout row references unknown group %q", row.Group)
			}
			for _, b := range row.Blocks {
				if b.Type == "" {
					return fmt.Errorf("layout block missing type")
				}
				if b.Ref != "" && (b.Type == "host" || b.Type == "guests" || b.Type == "containers" || b.Type == "metric") {
					if !hostNames[b.Ref] {
						return fmt.Errorf("layout block ref %q is not a known host", b.Ref)
					}
				}
			}
		}
	}
	return nil
}

func expandEnv(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(m string) string {
		sub := envPattern.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v, ok := os.LookupEnv(sub[1]); ok {
			return v
		}
		return m
	})
}

func EnsureFirstLaunch(dir string) (created bool, err error) {
	if _, err := FindAppFile(dir); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Join(dir, "apps.d"), 0o755); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "ssh"), 0o755); err != nil {
		return false, err
	}
	starter := starterYAML()
	if err := os.WriteFile(filepath.Join(dir, "app.yaml"), []byte(starter), 0o644); err != nil {
		return false, err
	}
	_ = os.WriteFile(filepath.Join(dir, "secrets.example.yaml"), []byte(secretsExample()), 0o644)
	readme := strings.TrimSpace(`
Dash config folder
==================

Edit app.yaml to list the machines you want to watch.

Dash already knows common apps (Home Assistant, Pi-hole, Jellyfin, …).
It finds them on your machines automatically. You do not manage those app files.

Optional:
  secrets.yaml     — API keys for apps that need them
  apps.d/          — only if you add a custom app Dash does not know yet

The web dashboard is read-only.
`) + "\n"
	_ = os.WriteFile(filepath.Join(dir, "README.txt"), []byte(readme), 0o644)
	return true, nil
}

func starterYAML() string {
	return `# Main settings for Dash.
# Edit this file. The dashboard reloads when the file changes.
# The web UI is read-only — change settings here, not in the browser.

poll_seconds: 5
history_hours: 24
listen: ":8080"

alerts:
  # Optional: https://ntfy.sh/your-topic
  ntfy_url: ""
  disk_percent: 90
  mem_percent: 95

# Top-level buckets shown as tabs on the board.
categories:
  - id: overview
    name: Overview

groups:
  - id: local
    name: This machine
    category: overview

# Screen layout: each row sets how many columns; blocks fill left to right.
layout:
  sections:
    - id: overview
      title: Your services
      category: overview
      rows:
        - columns: 3
          group: local
          blocks:
            - type: host
              ref: local
            - type: containers
              ref: local
            - type: unknown_apps

# Machines to watch. Add more blocks under hosts: as needed.
hosts:
  - name: local
    # Watch the Docker host this container runs on.
    # Mount /var/run/docker.sock:ro in compose for container stats.
    local: true
    icon: docker
    category: overview
    group: local
    tags: [local]
`
}

func secretsExample() string {
	return `# Optional API keys. Dash matches apps itself; put keys here by app id.
# Rename to secrets.yaml (or copy) and fill in. Do not commit real keys.
#
# homeassistant:
#   key: "your-long-lived-token"
# pihole:
#   key: "your-api-password"
# jellyfin:
#   key: "your-api-key"
`
}
