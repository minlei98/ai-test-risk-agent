package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRulesFileFromConfigDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	configDir := filepath.Join(root, "configs")
	rulesDir := filepath.Join(root, "rules")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	rulesFile := filepath.Join(rulesDir, "risk.yaml")
	if err := os.WriteFile(rulesFile, []byte("risk_categories: {}\ntest_types: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "osde2e.yaml")
	got, err := ResolveRulesFile(configPath, "")
	if err != nil {
		t.Fatalf("ResolveRulesFile: %v", err)
	}
	if got != rulesFile {
		t.Fatalf("expected %s, got %s", rulesFile, got)
	}
}

func TestResolveRulesFileExplicitRelativeToConfig(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "custom-rules.yaml")
	if err := os.WriteFile(custom, []byte("risk_categories: {}\ntest_types: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, nil, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveRulesFile(configPath, "custom-rules.yaml")
	if err != nil {
		t.Fatalf("ResolveRulesFile: %v", err)
	}
	if got != custom {
		t.Fatalf("expected %s, got %s", custom, got)
	}
}
