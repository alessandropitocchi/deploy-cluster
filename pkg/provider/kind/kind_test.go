package kind

import (
	"testing"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
)

func TestGenerateKindConfig_SingleNode(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "test",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       0,
		},
	}

	kindCfg := p.generateKindConfig(cfg)

	if kindCfg.Kind != "Cluster" {
		t.Errorf("Kind = %q, want %q", kindCfg.Kind, "Cluster")
	}
	if kindCfg.APIVersion != "kind.x-k8s.io/v1alpha4" {
		t.Errorf("APIVersion = %q, want %q", kindCfg.APIVersion, "kind.x-k8s.io/v1alpha4")
	}
	if len(kindCfg.Nodes) != 1 {
		t.Fatalf("Nodes count = %d, want 1", len(kindCfg.Nodes))
	}
	if kindCfg.Nodes[0].Role != "control-plane" {
		t.Errorf("Node[0].Role = %q, want %q", kindCfg.Nodes[0].Role, "control-plane")
	}
	if kindCfg.Nodes[0].Image != "" {
		t.Errorf("Node[0].Image = %q, want empty (no version specified)", kindCfg.Nodes[0].Image)
	}
}

func TestGenerateKindConfig_MultiNode(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "multi",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 3,
			Workers:       5,
		},
	}

	kindCfg := p.generateKindConfig(cfg)

	if len(kindCfg.Nodes) != 8 {
		t.Fatalf("Nodes count = %d, want 8 (3 cp + 5 workers)", len(kindCfg.Nodes))
	}

	cpCount := 0
	workerCount := 0
	for _, node := range kindCfg.Nodes {
		switch node.Role {
		case "control-plane":
			cpCount++
		case "worker":
			workerCount++
		default:
			t.Errorf("unexpected role: %q", node.Role)
		}
	}

	if cpCount != 3 {
		t.Errorf("control-plane count = %d, want 3", cpCount)
	}
	if workerCount != 5 {
		t.Errorf("worker count = %d, want 5", workerCount)
	}
}

func TestGenerateKindConfig_WithVersion(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "versioned",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       2,
			Version:       "v1.31.0",
		},
	}

	kindCfg := p.generateKindConfig(cfg)

	expectedImage := "kindest/node:v1.31.0"
	for i, node := range kindCfg.Nodes {
		if node.Image != expectedImage {
			t.Errorf("Node[%d].Image = %q, want %q", i, node.Image, expectedImage)
		}
	}
}

func TestGenerateKindConfig_NoVersion(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "no-version",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       1,
		},
	}

	kindCfg := p.generateKindConfig(cfg)

	for i, node := range kindCfg.Nodes {
		if node.Image != "" {
			t.Errorf("Node[%d].Image = %q, want empty", i, node.Image)
		}
	}
}

func TestGenerateKindConfig_ZeroWorkers(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "cp-only",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       0,
		},
	}

	kindCfg := p.generateKindConfig(cfg)

	if len(kindCfg.Nodes) != 1 {
		t.Fatalf("Nodes count = %d, want 1", len(kindCfg.Nodes))
	}
	if kindCfg.Nodes[0].Role != "control-plane" {
		t.Errorf("Node[0].Role = %q, want %q", kindCfg.Nodes[0].Role, "control-plane")
	}
}

func TestGenerateKindConfig_NodeOrder(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "order",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 2,
			Workers:       3,
		},
	}

	kindCfg := p.generateKindConfig(cfg)

	// Control planes should come first
	for i := 0; i < 2; i++ {
		if kindCfg.Nodes[i].Role != "control-plane" {
			t.Errorf("Node[%d].Role = %q, want control-plane (should come first)", i, kindCfg.Nodes[i].Role)
		}
	}
	for i := 2; i < 5; i++ {
		if kindCfg.Nodes[i].Role != "worker" {
			t.Errorf("Node[%d].Role = %q, want worker", i, kindCfg.Nodes[i].Role)
		}
	}
}

func TestGenerateKindConfig_WithIngress(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "ingress-test",
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

	kindCfg := p.generateKindConfig(cfg)

	cp := kindCfg.Nodes[0]
	// New ingress controllers don't need KubeadmConfigPatches
	if len(cp.ExtraPortMappings) != 2 {
		t.Fatalf("ExtraPortMappings count = %d, want 2", len(cp.ExtraPortMappings))
	}
	if cp.ExtraPortMappings[0].ContainerPort != 80 || cp.ExtraPortMappings[0].HostPort != 80 {
		t.Errorf("PortMapping[0] = %+v, want 80:80", cp.ExtraPortMappings[0])
	}
	if cp.ExtraPortMappings[1].ContainerPort != 443 || cp.ExtraPortMappings[1].HostPort != 443 {
		t.Errorf("PortMapping[1] = %+v, want 443:443", cp.ExtraPortMappings[1])
	}

	// Worker should NOT have ingress config
	worker := kindCfg.Nodes[1]
	if len(worker.KubeadmConfigPatches) != 0 {
		t.Errorf("Worker should not have KubeadmConfigPatches")
	}
	if len(worker.ExtraPortMappings) != 0 {
		t.Errorf("Worker should not have ExtraPortMappings")
	}
}

func TestGenerateKindConfig_WithoutIngress(t *testing.T) {
	p := New()
	cfg := &template.Template{
		Name: "no-ingress",
		Cluster: template.ClusterTemplate{
			ControlPlanes: 1,
			Workers:       1,
		},
	}

	kindCfg := p.generateKindConfig(cfg)

	cp := kindCfg.Nodes[0]
	if len(cp.KubeadmConfigPatches) != 0 {
		t.Errorf("Should not have KubeadmConfigPatches without ingress")
	}
	if len(cp.ExtraPortMappings) != 0 {
		t.Errorf("Should not have ExtraPortMappings without ingress")
	}
}

func TestKubeContext(t *testing.T) {
	p := New()
	got := p.KubeContext("my-cluster")
	want := "kind-my-cluster"
	if got != want {
		t.Errorf("KubeContext() = %q, want %q", got, want)
	}
}
