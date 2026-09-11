package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Type        string `yaml:"type"`
	} `yaml:"project"`
	Repositories []Repository `yaml:"repositories"`
	Analysis struct {
		MaxFileBytes   int      `yaml:"max_file_bytes"`
		MaxFiles       int      `yaml:"max_files"`
		Ignore         []string `yaml:"ignore"`
		TestExtensions []string `yaml:"test_extensions"`
		TestLevels     []string `yaml:"test_levels"`
		CrossCutting   []string `yaml:"cross_cutting"`
		CriticalTerms  []string `yaml:"critical_terms"`
		RulesFile      string   `yaml:"rules_file"`
	} `yaml:"analysis"`
	Risk struct {
		Weights         map[string]float64 `yaml:"weights"`
		Prioritize      []string           `yaml:"prioritize"`
		HighThreshold   float64            `yaml:"high_threshold"`
		MediumThreshold float64            `yaml:"medium_threshold"`
	} `yaml:"risk"`
	LLM struct {
		Enabled bool `yaml:"enabled"`
		BaseURL string `yaml:"base_url"`
		Model string `yaml:"model"`
		TimeoutSeconds int `yaml:"timeout_seconds"`
	} `yaml:"llm"`
	SourcePath string `yaml:"-"`
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
	abs, err := filepath.Abs(path)
	if err != nil { return nil, err }
	c.SourcePath = abs
	applyDefaults(&c)
	return &c, nil
}

func applyDefaults(c *Config) {
	if c.Analysis.MaxFileBytes == 0 {
		c.Analysis.MaxFileBytes = 1048576
	}
	if c.Analysis.MaxFiles == 0 {
		c.Analysis.MaxFiles = 20000
	}
	if len(c.Analysis.Ignore) == 0 {
		c.Analysis.Ignore = []string{".git", "vendor", "node_modules", "dist", "build", ".cache"}
	}
	if len(c.Analysis.TestExtensions) == 0 {
		c.Analysis.TestExtensions = []string{
			"_test.go", "_test.py", ".spec.ts", ".test.ts", ".spec.js", ".test.js", ".feature",
		}
	}
	if c.Risk.HighThreshold == 0 {
		c.Risk.HighThreshold = 70
	}
	if c.Risk.MediumThreshold == 0 {
		c.Risk.MediumThreshold = 40
	}
	for i := range c.Repositories {
		if c.Repositories[i].Branch == "" {
			c.Repositories[i].Branch = "main"
		}
		if c.Repositories[i].Role == "" {
			switch c.Repositories[i].Name {
			case "osde2e-common":
				c.Repositories[i].Role = "library"
			default:
				c.Repositories[i].Role = "application"
			}
		}
	}
}
