package startup

import (
	"fmt"

	"github.com/cego/gitte/config"
)

// selectChecks includes the requested checks and their transitive prerequisites.
func selectChecks(checks config.StartupCheckMap, names []string) (config.StartupCheckMap, error) {
	if len(names) == 0 {
		return checks, nil
	}
	selected := make(config.StartupCheckMap)
	var visit func(string) error
	visit = func(name string) error {
		if _, ok := selected[name]; ok {
			return nil
		}
		check, ok := checks[name]
		if !ok {
			return fmt.Errorf("unknown startup check %q", name)
		}
		selected[name] = check
		for _, dep := range check.GetNeeds() {
			if err := visit(dep); err != nil {
				return fmt.Errorf("startup check %q: %w", name, err)
			}
		}
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return selected, nil
}
