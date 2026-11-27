package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKubeContextSelection(t *testing.T) {
	const config = `apiVersion: v1
kind: Config
clusters:
- name: east
  cluster:
    server: https://east.example.test
- name: west
  cluster:
    server: https://west.example.test
contexts:
- name: east-preview
  context:
    cluster: east
- name: west-preview
  context:
    cluster: west
current-context: east-preview
`
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)

	for _, tt := range []struct{ context, alias, host string }{
		{"east-preview", "east-preview", "https://east.example.test"},
		{"west-preview", "west-preview", "https://west.example.test"},
		{"", "east-preview", "https://east.example.test"},
	} {
		t.Setenv("KUBE_CONTEXT", tt.context)
		cfg, parent, err := NewRestConfig()
		if err != nil {
			t.Fatalf("context %q: %v", tt.context, err)
		}
		if cfg.Host != tt.host || parent.Name != tt.alias || parent.InCluster {
			t.Errorf("context %q: host %q, parent %#v; want host %q, alias %q", tt.context, cfg.Host, parent, tt.host, tt.alias)
		}
	}

	t.Run("missing context", func(t *testing.T) {
		t.Setenv("KUBE_CONTEXT", "missing-preview")
		cfg, parent, err := NewRestConfig()
		if err == nil || !strings.Contains(err.Error(), "missing-preview") || cfg != nil || parent != (ParentConnection{}) {
			t.Fatalf("expected an error identifying the missing context; got config present %t, parent %#v, error %v", cfg != nil, parent, err)
		}
	})

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != config {
		t.Fatal("selecting a preview context modified the kubeconfig")
	}
}
