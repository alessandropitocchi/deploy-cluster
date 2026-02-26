package k3d

import (
	"testing"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
)

func TestGenerateK3dConfig_SingleNode(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "test",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       0,
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	if k3dCfg.Kind != "Simple" {
		t.Errorf("Kind = %q, want %q", k3dCfg.Kind, "Simple")
	}
	if k3dCfg.APIVersion != "k3d.io/v1alpha5" {
		t.Errorf("APIVersion = %q, want %q", k3dCfg.APIVersion, "k3d.io/v1alpha5")
	}
	if k3dCfg.Metadata.Name != "test" {
		t.Errorf("Metadata.Name = %q, want %q", k3dCfg.Metadata.Name, "test")
	}
	if k3dCfg.Servers != 1 {
		t.Errorf("Servers = %d, want 1", k3dCfg.Servers)
	}
	if k3dCfg.Agents != 0 {
		t.Errorf("Agents = %d, want 0", k3dCfg.Agents)
	}
	if k3dCfg.Image != "" {
		t.Errorf("Image = %q, want empty (no version specified)", k3dCfg.Image)
	}
	if len(k3dCfg.Ports) != 0 {
		t.Errorf("Ports count = %d, want 0 (no ingress)", len(k3dCfg.Ports))
	}
	if k3dCfg.Options != nil {
		t.Errorf("Options should be nil (no ingress)")
	}
}

func TestGenerateK3dConfig_MultiNode(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "multi",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 3,
			Workers:       5,
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	if k3dCfg.Servers != 3 {
		t.Errorf("Servers = %d, want 3", k3dCfg.Servers)
	}
	if k3dCfg.Agents != 5 {
		t.Errorf("Agents = %d, want 5", k3dCfg.Agents)
	}
}

func TestGenerateK3dConfig_WithVersion(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "versioned",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       2,
			Version:       "v1.31.0",
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	expectedImage := "rancher/k3s:v1.31.0-k3s1"
	if k3dCfg.Image != expectedImage {
		t.Errorf("Image = %q, want %q", k3dCfg.Image, expectedImage)
	}
}

func TestGenerateK3dConfig_NoVersion(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "no-version",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       1,
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	if k3dCfg.Image != "" {
		t.Errorf("Image = %q, want empty", k3dCfg.Image)
	}
}

func TestGenerateK3dConfig_WithIngressNginx(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "ingress-nginx",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       1,
		},
		Plugins: template.PluginsTemplate{
			Ingress: &template.IngressTemplate{
				Enabled: true,
				Type:    "nginx",
			},
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	// Should have port mappings on loadbalancer
	if len(k3dCfg.Ports) != 2 {
		t.Fatalf("Ports count = %d, want 2", len(k3dCfg.Ports))
	}
	if k3dCfg.Ports[0].Port != "80:80" {
		t.Errorf("Ports[0].Port = %q, want %q", k3dCfg.Ports[0].Port, "80:80")
	}
	if k3dCfg.Ports[0].NodeFilters[0] != "loadbalancer" {
		t.Errorf("Ports[0].NodeFilters[0] = %q, want %q", k3dCfg.Ports[0].NodeFilters[0], "loadbalancer")
	}
	if k3dCfg.Ports[1].Port != "443:443" {
		t.Errorf("Ports[1].Port = %q, want %q", k3dCfg.Ports[1].Port, "443:443")
	}

	// Should disable Traefik when using nginx
	if k3dCfg.Options == nil || k3dCfg.Options.K3s == nil {
		t.Fatal("Options.K3s should not be nil when using nginx")
	}
	if len(k3dCfg.Options.K3s.ExtraArgs) != 1 {
		t.Fatalf("ExtraArgs count = %d, want 1", len(k3dCfg.Options.K3s.ExtraArgs))
	}
	if k3dCfg.Options.K3s.ExtraArgs[0].Arg != "--disable=traefik" {
		t.Errorf("ExtraArgs[0].Arg = %q, want %q", k3dCfg.Options.K3s.ExtraArgs[0].Arg, "--disable=traefik")
	}
	if k3dCfg.Options.K3s.ExtraArgs[0].NodeFilters[0] != "server:*" {
		t.Errorf("ExtraArgs[0].NodeFilters[0] = %q, want %q", k3dCfg.Options.K3s.ExtraArgs[0].NodeFilters[0], "server:*")
	}
}

func TestGenerateK3dConfig_WithIngressTraefik(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "ingress-traefik",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       1,
		},
		Plugins: template.PluginsTemplate{
			Ingress: &template.IngressTemplate{
				Enabled: true,
				Type:    "traefik",
			},
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	// Should have port mappings
	if len(k3dCfg.Ports) != 2 {
		t.Fatalf("Ports count = %d, want 2", len(k3dCfg.Ports))
	}

	// Should NOT disable Traefik (it's built-in)
	if k3dCfg.Options != nil {
		t.Errorf("Options should be nil when using traefik (built-in, no need to disable)")
	}
}

func TestGenerateK3dConfig_WithoutIngress(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "no-ingress",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       1,
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	if len(k3dCfg.Ports) != 0 {
		t.Errorf("Ports count = %d, want 0 (no ingress)", len(k3dCfg.Ports))
	}
	if k3dCfg.Options != nil {
		t.Errorf("Options should be nil without ingress")
	}
}

func TestGenerateK3dConfig_IngressDisabled(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "ingress-disabled",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       1,
		},
		Plugins: template.PluginsTemplate{
			Ingress: &template.IngressTemplate{
				Enabled: false,
				Type:    "nginx",
			},
		},
	}

	k3dCfg := p.generateK3dConfig(cfg)

	if len(k3dCfg.Ports) != 0 {
		t.Errorf("Ports count = %d, want 0 (ingress disabled)", len(k3dCfg.Ports))
	}
}

func TestKubeContext(t *testing.T) {
	p := New()
	got := p.KubeContext("my-cluster")
	want := "k3d-my-cluster"
	if got != want {
		t.Errorf("KubeContext() = %q, want %q", got, want)
	}
}
