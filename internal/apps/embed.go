package apps

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// Built-in app definitions ship inside the binary. Users do not manage these.
// Optional overrides/additions go in config/apps.d/ only for custom apps.
//
//go:embed bundled/*.yaml
var bundledFS embed.FS

func LoadBundled() (map[string]*Def, []error) {
	out := map[string]*Def{}
	var errs []error
	err := fs.WalkDir(bundledFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		raw, err := bundledFS.ReadFile(path)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		var def Def
		if err := yaml.Unmarshal(raw, &def); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			return nil
		}
		if err := ValidateDef(&def); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			return nil
		}
		out[def.ID] = &def
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return out, errs
}

// LoadAll loads built-in defs from the binary, then optional user files in apps.d.
func LoadAll(userDir string) (map[string]*Def, []error) {
	out, errs := LoadBundled()
	user, uerrs := LoadDefs("", userDir)
	errs = append(errs, uerrs...)
	for id, def := range user {
		out[id] = def // user custom/override wins
	}
	return out, errs
}
