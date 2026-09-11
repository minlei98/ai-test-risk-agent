package rules

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Rules struct {
	RiskCategories map[string]RiskCategory `yaml:"risk_categories"`
	TestTypes      map[string][]string     `yaml:"test_types"`
}

type RiskCategory struct {
	Terms []string `yaml:"terms"`
	Base  int      `yaml:"base"`
}

func Load(path string) (*Rules, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Rules
	if err := yaml.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *Rules) CategoryTerms() map[string][]string {
	out := map[string][]string{}
	for name, cat := range r.RiskCategories {
		out[name] = cat.Terms
	}
	return out
}

func (r *Rules) TestTypePatterns(types []string) map[string][]string {
	if len(types) == 0 {
		return r.TestTypes
	}
	out := map[string][]string{}
	for _, t := range types {
		if p, ok := r.TestTypes[t]; ok {
			out[t] = p
		} else {
			out[t] = []string{t}
		}
	}
	return out
}
