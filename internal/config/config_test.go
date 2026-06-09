package config

import "testing"

func TestNormalizeBasePathDefaultsToClusternet(t *testing.T) {
	cfg := LoadFromEnv(map[string]string{})

	if cfg.BasePath != "/clusternet" {
		t.Fatalf("expected default base path /clusternet, got %q", cfg.BasePath)
	}
}

func TestNormalizeBasePathAddsLeadingSlashAndTrimsTrailingSlash(t *testing.T) {
	cfg := LoadFromEnv(map[string]string{"BASE_PATH": "clusternet/"})

	if cfg.BasePath != "/clusternet" {
		t.Fatalf("expected normalized base path /clusternet, got %q", cfg.BasePath)
	}
}

func TestNormalizeBasePathAllowsRoot(t *testing.T) {
	cfg := LoadFromEnv(map[string]string{"BASE_PATH": "/"})

	if cfg.BasePath != "" {
		t.Fatalf("expected root base path to normalize to empty prefix, got %q", cfg.BasePath)
	}
}
