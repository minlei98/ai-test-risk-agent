package llm

import (
	"fmt"
	"os"
	"path/filepath"
)

func LoadPrompts(dir string) (string, string, error) {
	systemPath := filepath.Join(dir, "system.md")
	reportPath := filepath.Join(dir, "report.md")

	systemBytes, err := os.ReadFile(systemPath)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", systemPath, err)
	}
	reportBytes, err := os.ReadFile(reportPath)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", reportPath, err)
	}
	return string(systemBytes), string(reportBytes), nil
}
