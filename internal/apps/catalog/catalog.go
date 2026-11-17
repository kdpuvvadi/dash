package catalog

import (
	"embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed popular.json
var popularFS embed.FS

type Entry struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Icon   string   `json:"icon"`
	Images []string `json:"images"`
	Names  []string `json:"names"`
}

type Match struct {
	Entry
	Source string // catalog | image | name | unknown
	Known  bool
}

var (
	once    sync.Once
	entries []Entry
	byID    map[string]*Entry
)

func load() {
	once.Do(func() {
		raw, err := popularFS.ReadFile("popular.json")
		if err != nil {
			return
		}
		_ = json.Unmarshal(raw, &entries)
		byID = make(map[string]*Entry, len(entries))
		for i := range entries {
			e := &entries[i]
			byID[e.ID] = e
		}
	})
}

func Count() int {
	load()
	return len(entries)
}

func All() []Entry {
	load()
	out := make([]Entry, len(entries))
	copy(out, entries)
	return out
}

func Get(id string) *Entry {
	load()
	return byID[id]
}

// Resolve identifies an app from a container image and/or name.
// Order: catalog image patterns → catalog name patterns → image path slug → name slug → unknown.
func Resolve(image, name string) Match {
	load()
	img := strings.ToLower(strings.TrimSpace(image))
	n := strings.ToLower(strings.TrimSpace(name))
	imgBase := imageBasename(img)
	nameBase := normalizeSlug(n)

	best := Match{Source: "unknown"}
	bestScore := 0

	for i := range entries {
		e := &entries[i]
		score := 0
		if imgBase != "" && (imgBase == e.ID || imgBase == normalizeSlug(e.Icon)) {
			score = max(score, 220)
		}
		if nameBase != "" && (nameBase == e.ID || nameBase == normalizeSlug(e.Icon)) {
			score = max(score, 200)
		}
		for _, p := range e.Images {
			p = strings.ToLower(p)
			if p == "" {
				continue
			}
			if img == p || strings.HasSuffix(img, "/"+p) || strings.Contains(img, "/"+p+":") || strings.Contains(img, "/"+p+"@") {
				score = max(score, 160+len(p))
			} else if strings.Contains(img, p) || imgBase == p || strings.Contains(imgBase, p) {
				score = max(score, 100+len(p))
			}
		}
		for _, p := range e.Names {
			p = strings.ToLower(p)
			if p == "" {
				continue
			}
			if n == p || nameBase == p {
				score = max(score, 140+len(p))
			} else if strings.Contains(n, p) {
				score = max(score, 80+len(p))
			}
		}
		if score > bestScore {
			bestScore = score
			best = Match{Entry: *e, Source: "catalog", Known: true}
		}
	}
	if best.Known {
		return best
	}

	// Fallback: derive icon/name from image path or container name.
	slug := imgBase
	if slug == "" || slug == "latest" || isGenericImage(slug) {
		slug = nameBase
	}
	slug = normalizeSlug(slug)
	if slug == "" {
		return Match{
			Entry:  Entry{ID: "unknown", Name: displayName(name, image), Icon: "docker"},
			Source: "unknown",
			Known:  false,
		}
	}
	if e := byID[slug]; e != nil {
		return Match{Entry: *e, Source: "catalog", Known: true}
	}
	// Strip common suffixes: -server, -web, -app, -ce, -ee
	for _, suf := range []string{"-server", "-web", "-app", "-ce", "-ee", "-backend", "-frontend", "-api", "-worker", "-agent", "-runner"} {
		if strings.HasSuffix(slug, suf) {
			base := strings.TrimSuffix(slug, suf)
			if e := byID[base]; e != nil {
				return Match{Entry: *e, Source: "catalog", Known: true}
			}
		}
	}

	return Match{
		Entry: Entry{
			ID:   slug,
			Name: titleCaseSlug(slug),
			Icon: slug,
		},
		Source: "image",
		Known:  true, // icon CDN may still 404 → UI falls back
	}
}

func imageBasename(image string) string {
	image = strings.Split(image, "@")[0]
	image = strings.Split(image, ":")[0]
	if i := strings.LastIndex(image, "/"); i >= 0 {
		image = image[i+1:]
	}
	return normalizeSlug(image)
}

func normalizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Trim(s, "-/")
	return s
}

func isGenericImage(slug string) bool {
	switch slug {
	case "latest", "stable", "main", "master", "release", "alpine", "scratch", "busybox", "ubuntu", "debian", "node", "python", "golang", "runtime":
		return true
	default:
		return false
	}
}

func displayName(name, image string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return imageBasename(image)
}

func titleCaseSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		switch p {
		case "js", "ui", "api", "ml", "ai", "tv", "vpn", "dns", "ssh", "pdf", "rss", "mqtt", "db", "io":
			parts[i] = strings.ToUpper(p)
		default:
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
