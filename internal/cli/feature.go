package cli

import (
	"fmt"
	"path"
	"strings"
)

// featureMap returns a map from feature basename (without the .md extension) to
// the configured repository-relative spec path. It ignores the master spec.
// An error is returned if two feature files share the same basename.
func (w *workspace) featureMap() (map[string]string, error) {
	paths, err := w.specPaths()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(paths)-1)
	for _, p := range paths {
		if p == w.cfg.Spec {
			continue
		}
		base := path.Base(p)
		if !strings.HasSuffix(base, ".md") {
			return nil, fmt.Errorf("feature spec %q must end in .md", p)
		}
		name := strings.TrimSuffix(base, ".md")
		if existing, ok := m[name]; ok {
			return nil, fmt.Errorf("feature name %q is ambiguous: matches both %q and %q", name, existing, p)
		}
		m[name] = p
	}
	return m, nil
}

// resolveFeature returns the configured spec path for a feature basename. An
// empty name selects the master spec. It returns an error if the name does not
// match any configured feature.
func (w *workspace) resolveFeature(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	m, err := w.featureMap()
	if err != nil {
		return "", err
	}
	p, ok := m[name]
	if !ok {
		names := make([]string, 0, len(m))
		for k := range m {
			names = append(names, k)
		}
		return "", fmt.Errorf("no feature named %q; configured features: %s", name, strings.Join(names, ", "))
	}
	return p, nil
}
