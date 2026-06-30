package main

import "testing"

func TestBuiltInConfigRunPattern(t *testing.T) {
	t.Parallel()

	cfg, err := builtInConfig("^missing Authorization$")
	if err != nil {
		t.Fatalf("builtInConfig returned error: %v", err)
	}
	if len(cfg.Probes) != 1 {
		t.Fatalf("builtInConfig returned %d probes, want 1", len(cfg.Probes))
	}
	if cfg.Probes[0].Name != "missing Authorization" {
		t.Fatalf("selected probe = %q, want %q", cfg.Probes[0].Name, "missing Authorization")
	}
}

func TestBuiltInConfigRejectsUnknownRunPattern(t *testing.T) {
	t.Parallel()

	_, err := builtInConfig("^does-not-exist$")
	if err == nil {
		t.Fatal("builtInConfig succeeded for an unknown run pattern")
	}
}

func TestParseOptionsRejectsPositionalArguments(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{"unexpected-positional-argument"})
	if err == nil {
		t.Fatal("parseOptions succeeded with positional arguments")
	}
}
