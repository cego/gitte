package config

// ResolveEnvWhen evaluates env_when entries against the given arch and returns
// only the variables whose conditions all pass. arch should be runtime.GOARCH.
// Unknown condition types conservatively exclude the variable.
func ResolveEnvWhen(entries map[string]EnvWhenEntry, arch string) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	result := make(map[string]string)
	for key, entry := range entries {
		if envWhenConditionsPass(entry.Conditions, arch) {
			result[key] = entry.Value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// envWhenConditionsPass returns true if all conditions pass (AND semantics).
// An empty list is treated as always-pass.
func envWhenConditionsPass(conditions []EnvWhenCondition, arch string) bool {
	for _, c := range conditions {
		if !envWhenConditionPass(c, arch) {
			return false
		}
	}
	return true
}

// envWhenConditionPass evaluates a single condition.
func envWhenConditionPass(c EnvWhenCondition, arch string) bool {
	switch c.Type {
	case "arch":
		for _, a := range c.Arch {
			if a == arch {
				return true
			}
		}
		return false
	default:
		// Unknown condition types conservatively exclude the variable.
		return false
	}
}
