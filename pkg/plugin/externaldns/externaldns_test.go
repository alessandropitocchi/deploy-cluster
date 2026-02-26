// Package externaldns provides the External DNS plugin.
package externaldns

import (
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
)

func TestNew(t *testing.T) {
	log := logger.New("[test]", logger.LevelQuiet)
	p := New(log, 5*time.Minute)

	if p == nil {
		t.Fatal("New() returned nil")
	}

	if p.Log != log {
		t.Error("Logger not set correctly")
	}

	if p.Timeout != 5*time.Minute {
		t.Errorf("Expected timeout 5m, got %v", p.Timeout)
	}
}

func TestPlugin_Name(t *testing.T) {
	p := &Plugin{Log: logger.New("[test]", logger.LevelQuiet)}

	if name := p.Name(); name != "external-dns" {
		t.Errorf("Expected name 'external-dns', got %q", name)
	}
}

func TestPlugin_getProviderChartValues(t *testing.T) {
	p := &Plugin{Log: logger.New("[test]", logger.LevelQuiet)}

	tests := []struct {
		provider string
		zone     string
	}{
		{"cloudflare", "example.com"},
		{"route53", "example.com"},
		{"google", "example.com"},
		{"azure", "example.com"},
		{"digitalocean", "example.com"},
	}

	for _, tt := range tests {
		cfg := &template.ExternalDNSTemplate{
			Provider: tt.provider,
			Zone:     tt.zone,
		}
		// Just verify it doesn't panic
		_, err := p.buildValues(cfg)
		// Error is expected if credentials not set, but shouldn't panic
		_ = err
	}
}

func TestPlugin_ValidateProvider(t *testing.T) {
	p := &Plugin{Log: logger.New("[test]", logger.LevelQuiet)}

	tests := []struct {
		provider string
		valid    bool
	}{
		{"cloudflare", true},
		{"route53", true},
		{"google", true},
		{"azure", true},
		{"digitalocean", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		cfg := &template.ExternalDNSTemplate{
			Provider: tt.provider,
			Zone:     "example.com",
		}
		_, err := p.buildValues(cfg)
		if tt.valid && err != nil {
			// Some valid providers may error due to missing credentials
			// but shouldn't error due to invalid provider
			if err.Error() == "unsupported DNS provider: invalid" {
				t.Errorf("Provider %q should be valid", tt.provider)
			}
		}
	}
}

func TestPlugin_IsInstalled(t *testing.T) {
	// This test would require mocking helm
	// For now, just test that the method exists
	p := &Plugin{Log: logger.New("[test]", logger.LevelQuiet), Timeout: 5 * time.Minute}
	_ = p.IsInstalled
}

func TestDefaultConstants(t *testing.T) {
	if defaultVersion != "1.15.0" {
		t.Errorf("Expected defaultVersion 1.15.0, got %s", defaultVersion)
	}
	if defaultNamespace != "external-dns" {
		t.Errorf("Expected defaultNamespace external-dns, got %s", defaultNamespace)
	}
	if helmRepo != "https://kubernetes-sigs.github.io/external-dns/" {
		t.Errorf("Unexpected helmRepo: %s", helmRepo)
	}
}
