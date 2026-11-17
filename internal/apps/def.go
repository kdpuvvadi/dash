package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Def struct {
	ID          string         `yaml:"id" json:"id"`
	Name        string         `yaml:"name" json:"name"`
	Credentials []Credential   `yaml:"credentials" json:"credentials"`
	Match       Match          `yaml:"match" json:"match"`
	Auth        Auth           `yaml:"auth" json:"auth"`
	Probe       *HTTPCall      `yaml:"probe" json:"probe,omitempty"`
	Collect     []CollectField `yaml:"collect" json:"collect"`
	UI          UI             `yaml:"ui" json:"ui"`
}

type Credential struct {
	Name     string `yaml:"name" json:"name"`
	Label    string `yaml:"label" json:"label"`
	Required bool   `yaml:"required" json:"required"`
}

type Match struct {
	DockerImage []string `yaml:"docker_image" json:"docker_image"`
	DockerName  []string `yaml:"docker_name" json:"docker_name"`
	Ports       []int    `yaml:"ports" json:"ports"`
}

type Auth struct {
	Type   string `yaml:"type" json:"type"` // bearer | header | query | basic | none
	Header string `yaml:"header" json:"header"`
	Value  string `yaml:"value" json:"value"`
	Query  string `yaml:"query" json:"query"`
}

type HTTPCall struct {
	Method   string `yaml:"method" json:"method"`
	Path     string `yaml:"path" json:"path"`
	Body     string `yaml:"body" json:"body"`
	OKStatus []int  `yaml:"ok_status" json:"ok_status"`
}

type CollectField struct {
	ID     string `yaml:"id" json:"id"`
	Label  string `yaml:"label" json:"label"`
	Method string `yaml:"method" json:"method"`
	Path   string `yaml:"path" json:"path"`
	Body   string `yaml:"body" json:"body"`
	Parse  string `yaml:"parse" json:"parse"` // jsonpath | text | regex
	Value  string `yaml:"value" json:"value"`
}

type UI struct {
	Icon   string   `yaml:"icon" json:"icon"`
	Fields []string `yaml:"fields" json:"fields"`
	Accent string   `yaml:"accent" json:"accent"`
}

// LoadDefs loads only from a directory (used for config/apps.d custom apps).
// Pass bundledDir as "" — built-ins come from LoadBundled / LoadAll.
func LoadDefs(bundledDir, userDir string) (map[string]*Def, []error) {
	out := map[string]*Def{}
	var errs []error
	loadDir := func(dir string) {
		if dir == "" {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				errs = append(errs, err)
			}
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}
			path := filepath.Join(dir, name)
			def, err := LoadDefFile(path)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, err))
				continue
			}
			out[def.ID] = def
		}
	}
	loadDir(bundledDir)
	loadDir(userDir)
	return out, errs
}

func LoadDefFile(path string) (*Def, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var def Def
	if err := yaml.Unmarshal(raw, &def); err != nil {
		return nil, err
	}
	if err := ValidateDef(&def); err != nil {
		return nil, err
	}
	return &def, nil
}

func ValidateDef(d *Def) error {
	if d.ID == "" {
		return fmt.Errorf("id is required")
	}
	if d.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(d.Collect) == 0 && d.Probe == nil {
		return fmt.Errorf("collect or probe is required")
	}
	for _, c := range d.Collect {
		if c.ID == "" || c.Path == "" {
			return fmt.Errorf("collect entries need id and path")
		}
		if c.Parse == "" {
			c.Parse = "text"
		}
	}
	if d.Auth.Type == "" {
		d.Auth.Type = "none"
	}
	return nil
}

func (d *Def) Matches(image, name string, ports []int) bool {
	img := strings.ToLower(image)
	n := strings.ToLower(name)
	for _, p := range d.Match.DockerImage {
		if strings.Contains(img, strings.ToLower(p)) {
			return true
		}
	}
	for _, p := range d.Match.DockerName {
		if strings.Contains(n, strings.ToLower(p)) {
			return true
		}
	}
	// Port-only matching is too noisy when image/name patterns exist
	// (e.g. AdGuard on :80 was matching Pi-hole). Use ports alone only
	// when the def has no image or name hints.
	if len(d.Match.DockerImage) == 0 && len(d.Match.DockerName) == 0 && len(d.Match.Ports) > 0 {
		set := map[int]bool{}
		for _, p := range ports {
			set[p] = true
		}
		for _, p := range d.Match.Ports {
			if set[p] {
				return true
			}
		}
	}
	return false
}

func DefsJSON(defs map[string]*Def) []byte {
	list := make([]*Def, 0, len(defs))
	for _, d := range defs {
		list = append(list, d)
	}
	b, _ := json.Marshal(list)
	return b
}
