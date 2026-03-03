package ingress

import (
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
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
	want := "unsupported ingress type: haproxy (supported: traefik, nginx-gateway-fabric)"
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestUninstall_UnsupportedType(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "haproxy"}
	err := p.Uninstall(cfg, "fake-context")
	if err == nil {
		t.Fatal("Uninstall() should fail for unsupported type")
	}
	want := "unsupported ingress type: haproxy"
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
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
	want := "unsupported ingress type: traefik (supported: traefik, nginx-gateway-fabric)"
	if err != nil && err.Error() == want {
		t.Error("traefik should be a supported ingress type")
	}
}

func TestInstall_NginxGatewayFabricType(t *testing.T) {
	// NGINX Gateway Fabric type should be accepted
	p := New(testLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "nginx-gateway-fabric"}
	err := p.Install(cfg, "fake-context", "kind")
	// Will fail because no real cluster, but should NOT be "unsupported ingress type"
	want := "unsupported ingress type: nginx-gateway-fabric (supported: traefik, nginx-gateway-fabric)"
	if err != nil && err.Error() == want {
		t.Error("nginx-gateway-fabric should be a supported ingress type")
	}
}
