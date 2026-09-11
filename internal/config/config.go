package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	} `yaml:"project"`
	Repositories []Repository `yaml:"repositories"`
	Analysis struct {
		MaxFileBytes int      `yaml:"max_file_bytes"`
		MaxFiles     int      `yaml:"max_files"`
		Ignore       []string `yaml:"ignore"`
		TestExtensions []string `yaml:"test_extensions"`
		CriticalTerms []string `yaml:"critical_terms"`
	} `yaml:"analysis"`
	Risk struct {
		Weights map[string]float64 `yaml:"weights"`
		HighThreshold float64 `yaml:"high_threshold"`
		MediumThreshold float64 `yaml:"medium_threshold"`
	} `yaml:"risk"`
	LLM struct {
		Enabled bool `yaml:"enabled"`
		BaseURL string `yaml:"base_url"`
		Model string `yaml:"model"`
		TimeoutSeconds int `yaml:"timeout_seconds"`
	} `yaml:"llm"`
}

type Repository struct {
	Name string `yaml:"name"`
	URL string `yaml:"url"`
	Branch string `yaml:"branch"`
	Role string `yaml:"role"`
	Path string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil { return nil, err }
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil { return nil, err }
	return &c, nil
}
