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
		if c.Hosts[i].Category == "" {
			c.Hosts[i].Category = "overview"
		}
		if c.Hosts[i].Group == "" {
			c.Hosts[i].Group = c.Hosts[i].Name
		}
	}
	c.ensureTaxonomy()
	c.ensureLayout()
}

// ensureTaxonomy derives categories/groups from host fields so app.yaml
// only needs category/group on each host — no separate lists required.
func (c *Config) ensureTaxonomy() {
	catIDs := map[string]bool{}
	for _, cat := range c.Categories {
		if cat.ID != "" {
			catIDs[cat.ID] = true
		}
	}
	groupIDs := map[string]bool{}
	for _, g := range c.Groups {
		if g.ID != "" {
			groupIDs[g.ID] = true
		}
	}
	for _, h := range c.Hosts {
		if h.Category != "" && !catIDs[h.Category] {
			c.Categories = append(c.Categories, Category{ID: h.Category, Name: humanizeID(h.Category)})
			catIDs[h.Category] = true
		}
		if h.Group != "" && !groupIDs[h.Group] {
			c.Groups = append(c.Groups, Group{ID: h.Group, Name: humanizeID(h.Group), Category: h.Category})
			groupIDs[h.Group] = true
		}
	}
	if len(c.Categories) == 0 {
		c.Categories = []Category{{ID: "overview", Name: "Overview"}}
	}
}

// ensureLayout adds a host+containers row for every host not already in layout.
func (c *Config) ensureLayout() {
	referenced := map[string]bool{}
	hasUnknown := false
	for _, sec := range c.Layout.Sections {
		for _, row := range sec.Rows {
			for _, b := range row.Blocks {
				if b.Type == "unknown_apps" {
					hasUnknown = true
				}
				if b.Ref != "" && (b.Type == "host" || b.Type == "containers" || b.Type == "guests" || b.Type == "metric") {
					referenced[b.Ref] = true
				}
			}
		}
	}
	if len(c.Layout.Sections) == 0 {
		c.Layout.Sections = []Section{{
			ID:       "overview",
			Title:    "Your services",
			Category: "overview",
		}}
	}
	secByCat := map[string]int{}
	for i, sec := range c.Layout.Sections {
		cat := sec.Category
		if cat == "" {
			cat = "overview"
		}
		secByCat[cat] = i
	}
	for _, h := range c.Hosts {
		if referenced[h.Name] {
			continue
		}
		cat := h.Category
		if cat == "" {
			cat = "overview"
		}
		idx, ok := secByCat[cat]
		if !ok {
			c.Layout.Sections = append(c.Layout.Sections, Section{
				ID:       cat,
				Title:    humanizeID(cat),
				Category: cat,
			})
			idx = len(c.Layout.Sections) - 1
			secByCat[cat] = idx
		}
		row := Row{
			Columns: 1,
			Group:   h.Group,
			Blocks: []Block{
				{Type: "host", Ref: h.Name, Icon: h.Icon},
				{Type: "containers", Ref: h.Name},
			},
		}
		c.Layout.Sections[idx].Rows = append(c.Layout.Sections[idx].Rows, row)
		referenced[h.Name] = true
	}
	if !hasUnknown && len(c.Layout.Sections) > 0 {
		c.Layout.Sections[0].Rows = append(c.Layout.Sections[0].Rows, Row{
			Columns: 1,
			Blocks:  []Block{{Type: "unknown_apps"}},
		})
	}
}

func humanizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

func (c *Config) Validate() error {
	hostNames := map[string]bool{}
	for _, h := range c.Hosts {
		if h.Name == "" {
			return fmt.Errorf("host missing name")
		}
		if !h.Local && h.Host == "" {
			return fmt.Errorf("host %q needs host or local: true", h.Name)
		}
		hostNames[h.Name] = true
	}
	for _, cat := range c.Categories {
		if cat.ID == "" {
			return fmt.Errorf("category missing id")
		}
	}
	for _, g := range c.Groups {
		if g.ID == "" {
			return fmt.Errorf("group missing id")
		}
	}
	for si := range c.Layout.Sections {
		for ri := range c.Layout.Sections[si].Rows {
			row := &c.Layout.Sections[si].Rows[ri]
			if row.Columns <= 0 {
				row.Columns = 1
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

# Machines to watch. category/group on the host are enough —
# tabs and sections are derived automatically. Layout is optional.
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
