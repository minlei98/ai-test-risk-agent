package config

import "testing"

func TestCategoryPriorityDeprioritizesPerformance(t *testing.T) {
	cfg := &Config{}
	cfg.Risk.Deprioritize = []string{"performance"}
	cfg.Risk.Prioritize = []string{"security", "e2e"}

	order := cfg.CategoryPriority()
	if order[len(order)-1] != "performance" {
		t.Fatalf("expected performance last, got order %v", order)
	}
	if order[0] != "security" {
		t.Fatalf("expected security first, got order %v", order)
	}
}

func TestIsDeprioritized(t *testing.T) {
	cfg := &Config{}
	cfg.Risk.Deprioritize = []string{"performance"}
	if !cfg.IsDeprioritized("performance") {
		t.Fatal("expected performance to be deprioritized")
	}
	if cfg.IsDeprioritized("security") {
		t.Fatal("security should not be deprioritized")
	}
}
