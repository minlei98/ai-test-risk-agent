package config

import "strings"

var defaultCategoryPriority = []string{
	"security", "resilience", "upgrade", "negative", "integration", "e2e", "performance",
}

// CategoryPriority returns test-category precedence for Jira classification.
func (c *Config) CategoryPriority() []string {
	order := append([]string{}, defaultCategoryPriority...)
	if c == nil {
		return order
	}

	moveFront := func(cat string) {
		cat = strings.ToLower(strings.TrimSpace(cat))
		for i, item := range order {
			if item == cat {
				order = append(order[:i], order[i+1:]...)
				order = append([]string{cat}, order...)
				return
			}
		}
	}
	moveBack := func(cat string) {
		cat = strings.ToLower(strings.TrimSpace(cat))
		for i, item := range order {
			if item == cat {
				order = append(order[:i], order[i+1:]...)
				order = append(order, cat)
				return
			}
		}
	}

	for i := len(c.Risk.Prioritize) - 1; i >= 0; i-- {
		moveFront(strings.ToLower(strings.TrimSpace(c.Risk.Prioritize[i])))
	}
	for _, cat := range c.Risk.Deprioritize {
		moveBack(strings.ToLower(strings.TrimSpace(cat)))
	}
	return order
}

// IsDeprioritized reports whether a test category is intentionally lower priority.
func (c *Config) IsDeprioritized(category string) bool {
	if c == nil {
		return false
	}
	category = strings.ToLower(strings.TrimSpace(category))
	for _, cat := range c.Risk.Deprioritize {
		if strings.ToLower(strings.TrimSpace(cat)) == category {
			return true
		}
	}
	return false
}
