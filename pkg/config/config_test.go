package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Name != "my-cluster" {
		t.Errorf("Name = %q, want %q", cfg.Name, "my-cluster")
	}
	if cfg.Provider.Type != "kind" {
		t.Errorf("Provider.Type = %q, want %q", cfg.Provider.Type, "kind")
	}
	if cfg.Cluster.ControlPlanes != 1 {
		t.Errorf("ControlPlanes = %d, want 1", cfg.Cluster.ControlPlanes)
	}
	if cfg.Cluster.Workers != 2 {
		t.Errorf("Workers = %d, want 2", cfg.Cluster.Workers)
	}
	if cfg.Cluster.Version != "v1.31.0" {
		t.Errorf("Version = %q, want %q", cfg.Cluster.Version, "v1.31.0")
	}
	if cfg.Plugins.ArgoCD == nil {
		t.Fatal("ArgoCD config should not be nil")
	}
	if cfg.Plugins.ArgoCD.Enabled {
		t.Error("ArgoCD should be disabled by default")
	}
	if len(cfg.Plugins.ArgoCD.Repos) != 1 {
		t.Errorf("Repos count = %d, want 1", len(cfg.Plugins.ArgoCD.Repos))
	}
	if len(cfg.Plugins.ArgoCD.Apps) != 1 {
		t.Errorf("Apps count = %d, want 1", len(cfg.Plugins.ArgoCD.Apps))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-cluster.yaml")

	original := &Config{
		Name: "test-cluster",
		Provider: ProviderConfig{
			Type: "kind",
		},
		Cluster: ClusterConfig{
			ControlPlanes: 3,
			Workers:       5,
			Version:       "v1.30.0",
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Provider.Type != original.Provider.Type {
		t.Errorf("Provider.Type = %q, want %q", loaded.Provider.Type, original.Provider.Type)
	}
	if loaded.Cluster.ControlPlanes != original.Cluster.ControlPlanes {
		t.Errorf("ControlPlanes = %d, want %d", loaded.Cluster.ControlPlanes, original.Cluster.ControlPlanes)
	}
	if loaded.Cluster.Workers != original.Cluster.Workers {
		t.Errorf("Workers = %d, want %d", loaded.Cluster.Workers, original.Cluster.Workers)
	}
	if loaded.Cluster.Version != original.Cluster.Version {
		t.Errorf("Version = %q, want %q", loaded.Cluster.Version, original.Cluster.Version)
	}
}

func TestSaveAndLoad_WithPlugins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-plugins.yaml")

	autoSync := true
	original := &Config{
		Name:     "plugin-cluster",
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: 1},
		Plugins: PluginsConfig{
			ArgoCD: &ArgoCDConfig{
				Enabled:   true,
				Namespace: "argocd",
				Version:   "v2.12.0",
				Repos: []ArgoCDRepoConfig{
					{
						Name: "my-repo",
						URL:  "https://github.com/user/repo.git",
						Type: "git",
					},
					{
						Name:       "private-repo",
						URL:        "git@github.com:user/private.git",
						Type:       "git",
						SSHKeyEnv:  "SSH_KEY",
					},
				},
				Apps: []ArgoCDAppConfig{
					{
						Name:           "my-app",
						Namespace:      "apps",
						RepoURL:        "https://charts.example.com",
						Chart:          "my-chart",
						TargetRevision: "1.0.0",
						AutoSync:       &autoSync,
						Values: map[string]interface{}{
							"replicas": 3,
						},
					},
				},
			},
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Plugins.ArgoCD == nil {
		t.Fatal("ArgoCD config should not be nil")
	}
	if !loaded.Plugins.ArgoCD.Enabled {
		t.Error("ArgoCD should be enabled")
	}
	if loaded.Plugins.ArgoCD.Version != "v2.12.0" {
		t.Errorf("ArgoCD Version = %q, want %q", loaded.Plugins.ArgoCD.Version, "v2.12.0")
	}
	if len(loaded.Plugins.ArgoCD.Repos) != 2 {
		t.Fatalf("Repos count = %d, want 2", len(loaded.Plugins.ArgoCD.Repos))
	}
	if loaded.Plugins.ArgoCD.Repos[0].Name != "my-repo" {
		t.Errorf("Repo[0].Name = %q, want %q", loaded.Plugins.ArgoCD.Repos[0].Name, "my-repo")
	}
	if loaded.Plugins.ArgoCD.Repos[1].SSHKeyEnv != "SSH_KEY" {
		t.Errorf("Repo[1].SSHKeyEnv = %q, want %q", loaded.Plugins.ArgoCD.Repos[1].SSHKeyEnv, "SSH_KEY")
	}
	if len(loaded.Plugins.ArgoCD.Apps) != 1 {
		t.Fatalf("Apps count = %d, want 1", len(loaded.Plugins.ArgoCD.Apps))
	}
	app := loaded.Plugins.ArgoCD.Apps[0]
	if app.Name != "my-app" {
		t.Errorf("App.Name = %q, want %q", app.Name, "my-app")
	}
	if app.Chart != "my-chart" {
		t.Errorf("App.Chart = %q, want %q", app.Chart, "my-chart")
	}
	if app.AutoSync == nil || !*app.AutoSync {
		t.Error("App.AutoSync should be true")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/cluster.yaml")
	if err == nil {
		t.Error("Load() should return error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() should return error for invalid YAML")
	}
}

func TestLoad_EmptyPlugins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minimal.yaml")

	content := `name: minimal
provider:
  type: kind
cluster:
  controlPlanes: 1
  workers: 0
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Name != "minimal" {
		t.Errorf("Name = %q, want %q", cfg.Name, "minimal")
	}
	if cfg.Plugins.ArgoCD != nil {
		t.Error("ArgoCD should be nil when not specified")
	}
	if cfg.Cluster.Workers != 0 {
		t.Errorf("Workers = %d, want 0", cfg.Cluster.Workers)
	}
}

// --- Validation tests ---

func TestValidate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should pass for default config, got: %v", err)
	}
}

func TestValidate_EmptyName(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: 0},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for empty name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error should mention name, got: %v", err)
	}
}

func TestValidate_EmptyProviderType(t *testing.T) {
	cfg := &Config{
		Name:    "test",
		Cluster: ClusterConfig{ControlPlanes: 1, Workers: 0},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for empty provider type")
	}
	if !strings.Contains(err.Error(), "provider.type is required") {
		t.Errorf("error should mention provider.type, got: %v", err)
	}
}

func TestValidate_InvalidProviderType(t *testing.T) {
	cfg := &Config{
		Name:     "test",
		Provider: ProviderConfig{Type: "docker-desktop"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: 0},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for invalid provider type")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error should mention not supported, got: %v", err)
	}
}

func TestValidate_ZeroControlPlanes(t *testing.T) {
	cfg := &Config{
		Name:     "test",
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 0, Workers: 1},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for 0 control planes")
	}
	if !strings.Contains(err.Error(), "controlPlanes must be at least 1") {
		t.Errorf("error should mention controlPlanes, got: %v", err)
	}
}

func TestValidate_NegativeWorkers(t *testing.T) {
	cfg := &Config{
		Name:     "test",
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: -1},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for negative workers")
	}
	if !strings.Contains(err.Error(), "workers cannot be negative") {
		t.Errorf("error should mention workers, got: %v", err)
	}
}

func TestValidate_ArgoCDAppMissingName(t *testing.T) {
	cfg := &Config{
		Name:     "test",
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: 0},
		Plugins: PluginsConfig{
			ArgoCD: &ArgoCDConfig{
				Enabled: true,
				Apps: []ArgoCDAppConfig{
					{RepoURL: "https://example.com"},
				},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for app without name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error should mention app name, got: %v", err)
	}
}

func TestValidate_ArgoCDAppMissingRepoURL(t *testing.T) {
	cfg := &Config{
		Name:     "test",
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: 0},
		Plugins: PluginsConfig{
			ArgoCD: &ArgoCDConfig{
				Enabled: true,
				Apps: []ArgoCDAppConfig{
					{Name: "my-app"},
				},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for app without repoURL")
	}
	if !strings.Contains(err.Error(), "repoURL is required") {
		t.Errorf("error should mention repoURL, got: %v", err)
	}
}

func TestValidate_ArgoCDRepoMissingURL(t *testing.T) {
	cfg := &Config{
		Name:     "test",
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: 0},
		Plugins: PluginsConfig{
			ArgoCD: &ArgoCDConfig{
				Enabled: true,
				Repos: []ArgoCDRepoConfig{
					{Name: "no-url"},
				},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail for repo without url")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("error should mention url, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &Config{} // everything missing
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail")
	}
	// Should report name, provider.type, and controlPlanes errors
	if !strings.Contains(err.Error(), "name is required") {
		t.Error("should report name error")
	}
	if !strings.Contains(err.Error(), "provider.type is required") {
		t.Error("should report provider.type error")
	}
	if !strings.Contains(err.Error(), "controlPlanes must be at least 1") {
		t.Error("should report controlPlanes error")
	}
}

func TestValidate_DisabledArgoCDSkipsValidation(t *testing.T) {
	cfg := &Config{
		Name:     "test",
		Provider: ProviderConfig{Type: "kind"},
		Cluster:  ClusterConfig{ControlPlanes: 1, Workers: 0},
		Plugins: PluginsConfig{
			ArgoCD: &ArgoCDConfig{
				Enabled: false,
				Apps:    []ArgoCDAppConfig{{Name: ""}}, // invalid but disabled
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should skip ArgoCD validation when disabled, got: %v", err)
	}
}

func TestLoad_ValidationFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yaml")

	content := `name: ""
provider:
  type: kind
cluster:
  controlPlanes: 0
  workers: 0
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() should fail when validation fails")
	}
}
