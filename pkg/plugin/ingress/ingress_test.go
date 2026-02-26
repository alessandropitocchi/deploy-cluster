package ingress

import (
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
)

func testLogger() *logger.Logger {
	return logger.New("[ingress]", logger.LevelQuiet)
}

func TestNew(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	if p == nil {
		t.Fatal("New() should return non-nil plugin")
	}
	if p.Log == nil {
		t.Error("Log should not be nil")
	}
	if p.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want %v", p.Timeout, 5*time.Minute)
	}
}

func TestNew_CustomTimeout(t *testing.T) {
	p := New(testLogger(), 30*time.Second)
	if p.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", p.Timeout, 30*time.Second)
	}
}

func TestName(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	if got := p.Name(); got != "ingress" {
		t.Errorf("Name() = %q, want %q", got, "ingress")
	}
}

func TestInstall_UnsupportedType(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "haproxy"}
	err := p.Install(cfg, "fake-context", "kind")
	if err == nil {
		t.Fatal("Install() should fail for unsupported type")
	}
	if got := err.Error(); got != "unsupported ingress type: haproxy (supported: nginx, traefik)" {
		t.Errorf("error = %q, want specific message", got)
	}
}

func TestUninstall_UnsupportedType(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "haproxy"}
	err := p.Uninstall(cfg, "fake-context")
	if err == nil {
		t.Fatal("Uninstall() should fail for unsupported type")
	}
	if got := err.Error(); got != "unsupported ingress type: haproxy" {
		t.Errorf("error = %q, want specific message", got)
	}
}

func TestInstall_TraefikType(t *testing.T) {
	// Traefik type should be accepted (not unsupported)
	// We can't fully test installation without a real cluster,
	// but we verify the type routing doesn't error as "unsupported"
	p := New(testLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "traefik"}
	err := p.Install(cfg, "fake-context", "k3d")
	// Will fail because no real cluster, but should NOT be "unsupported ingress type"
	if err != nil && err.Error() == "unsupported ingress type: traefik (supported: nginx, traefik)" {
		t.Error("traefik should be a supported ingress type")
	}
}

func TestNginxManifestURL_Kind(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	got := p.nginxManifestURL("kind")
	if got != nginxManifestKindURL {
		t.Errorf("nginxManifestURL(kind) = %q, want kind URL", got)
	}
}

func TestNginxManifestURL_K3d(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	got := p.nginxManifestURL("k3d")
	if got != nginxManifestCloudURL {
		t.Errorf("nginxManifestURL(k3d) = %q, want cloud URL", got)
	}
}
