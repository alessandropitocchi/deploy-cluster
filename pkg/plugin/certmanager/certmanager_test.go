package certmanager

import (
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
)

func testLogger() *logger.Logger {
	return logger.New("[cert-manager]", logger.LevelQuiet)
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
	if got := p.Name(); got != "cert-manager" {
		t.Errorf("Name() = %q, want %q", got, "cert-manager")
	}
}

func TestManifestURL_DefaultVersion(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	url := p.manifestURL("v1.16.3")
	want := "https://github.com/cert-manager/cert-manager/releases/download/v1.16.3/cert-manager.yaml"
	if url != want {
		t.Errorf("manifestURL() = %q, want %q", url, want)
	}
}

func TestManifestURL_CustomVersion(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	url := p.manifestURL("v1.14.0")
	want := "https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml"
	if url != want {
		t.Errorf("manifestURL() = %q, want %q", url, want)
	}
}
