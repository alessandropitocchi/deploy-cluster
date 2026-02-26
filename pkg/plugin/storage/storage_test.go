package storage

import (
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
)

func testLogger() *logger.Logger {
	return logger.New("[storage]", logger.LevelQuiet)
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
	if got := p.Name(); got != "storage" {
		t.Errorf("Name() = %q, want %q", got, "storage")
	}
}

func TestInstall_UnsupportedType(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	cfg := &template.StorageTemplate{Enabled: true, Type: "openebs"}
	err := p.Install(cfg, "fake-context", "kind")
	if err == nil {
		t.Fatal("Install() should fail for unsupported type")
	}
	if got := err.Error(); got != "unsupported storage type: openebs (supported: local-path)" {
		t.Errorf("error = %q, want specific message", got)
	}
}

func TestUninstall_UnsupportedType(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	cfg := &template.StorageTemplate{Enabled: true, Type: "openebs"}
	err := p.Uninstall(cfg, "fake-context")
	if err == nil {
		t.Fatal("Uninstall() should fail for unsupported type")
	}
	if got := err.Error(); got != "unsupported storage type: openebs" {
		t.Errorf("error = %q, want specific message", got)
	}
}
