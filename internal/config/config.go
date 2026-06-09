package config

import "strings"

const defaultBasePath = "/clusternet"

// Config holds runtime configuration for the dashboard server.
type Config struct {
	BasePath  string
	Port      string
	StaticDir string
}

// LoadFromEnv creates Config from an environment map. It is intentionally map-based
// so tests can exercise configuration behavior without mutating process env.
func LoadFromEnv(env map[string]string) Config {
	port := env["PORT"]
	if port == "" {
		port = "8080"
	}

	basePath := env["BASE_PATH"]
	if basePath == "" {
		basePath = defaultBasePath
	}

	staticDir := env["STATIC_DIR"]
	if staticDir == "" {
		staticDir = "web/dist"
	}

	return Config{
		BasePath:  NormalizeBasePath(basePath),
		Port:      port,
		StaticDir: staticDir,
	}
}

// NormalizeBasePath returns an HTTP path prefix suitable for routing.
// Root deployments normalize to an empty prefix so routes become /api/... .
func NormalizeBasePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return ""
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	return path
}
