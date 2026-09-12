package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadPrompts(dir string, mode string) (string, string, error) {
	systemPath := filepath.Join(dir, "system.md")
	reportName := "report.md"
	if strings.EqualFold(mode, "executive") {
		reportName = "report-executive.md"
	}
	reportPath := filepath.Join(dir, reportName)

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
