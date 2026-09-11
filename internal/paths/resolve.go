package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveRulesFile locates rules/risk.yaml using the config file location as anchor.
// If rulesFile is non-empty it is resolved relative to the config directory unless absolute.
// Otherwise the search walks upward from the config directory for rules/risk.yaml.
func ResolveRulesFile(configPath, rulesFile string) (string, error) {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	configDir := filepath.Dir(configPath)

	if rulesFile != "" {
		if filepath.IsAbs(rulesFile) {
			return rulesFile, nil
		}
		return filepath.Join(configDir, rulesFile), nil
	}

	if path, err := findUp(configDir, "rules", "risk.yaml"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("rules/risk.yaml not found near config %s", configPath)
}

// ResolveDir locates a directory (e.g. prompts/) relative to the config file.
func ResolveDir(configPath, dir string) (string, error) {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	if filepath.IsAbs(dir) {
		return dir, nil
	}
	configDir := filepath.Dir(configPath)
	if path, err := findUp(configDir, dir); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found near config %s", dir, configPath)
}

func findUp(start string, elem ...string) (string, error) {
	dir := start
	for {
		candidate := filepath.Join(dir, filepath.Join(elem...))
		if info, err := os.Stat(candidate); err == nil {
			if len(elem) == 1 {
				if !info.IsDir() {
					return "", fmt.Errorf("%s is not a directory", candidate)
				}
			}
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("%s not found from %s", filepath.Join(elem...), start)
}
