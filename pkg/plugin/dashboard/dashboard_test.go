package dashboard

import (
	"testing"

	"github.com/alepito/deploy-cluster/pkg/config"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() should return non-nil plugin")
	}
	if !p.Verbose {
		t.Error("Verbose should default to true")
	}
}

func TestName(t *testing.T) {
	p := New()
	if got := p.Name(); got != "dashboard" {
		t.Errorf("Name() = %q, want %q", got, "dashboard")
	}
}

func TestInstall_UnsupportedType(t *testing.T) {
	p := New()
	p.Verbose = false
	cfg := &config.DashboardConfig{Enabled: true, Type: "lens"}
	err := p.Install(cfg, "fake-context")
	if err == nil {
		t.Fatal("Install() should fail for unsupported type")
	}
	if got := err.Error(); got != "unsupported dashboard type: lens (supported: headlamp)" {
		t.Errorf("error = %q, want specific message", got)
	}
}

func TestUninstall_UnsupportedType(t *testing.T) {
	p := New()
	p.Verbose = false
	cfg := &config.DashboardConfig{Enabled: true, Type: "lens"}
	err := p.Uninstall(cfg, "fake-context")
	if err == nil {
		t.Fatal("Uninstall() should fail for unsupported type")
	}
	if got := err.Error(); got != "unsupported dashboard type: lens" {
		t.Errorf("error = %q, want specific message", got)
	}
}

func TestChartVersion_Default(t *testing.T) {
	p := New()
	cfg := &config.DashboardConfig{Enabled: true, Type: "headlamp"}
	if got := p.chartVersion(cfg); got != defaultHeadlampVersion {
		t.Errorf("chartVersion() = %q, want %q", got, defaultHeadlampVersion)
	}
}

func TestChartVersion_Custom(t *testing.T) {
	p := New()
	cfg := &config.DashboardConfig{Enabled: true, Type: "headlamp", Version: "0.20.0"}
	if got := p.chartVersion(cfg); got != "0.20.0" {
		t.Errorf("chartVersion() = %q, want %q", got, "0.20.0")
	}
}

func TestLog_Silent(t *testing.T) {
	p := New()
	p.Verbose = false
	p.log("test %s\n", "message")
}
