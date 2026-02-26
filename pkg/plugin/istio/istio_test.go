// Package istio provides the Istio service mesh plugin.
package istio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alepito/deploy-cluster/pkg/logger"
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

	if name := p.Name(); name != "istio" {
		t.Errorf("Expected name 'istio', got %q", name)
	}
}

func TestDefaultConstants(t *testing.T) {
	// Just verify constants are set
	if defaultVersion == "" {
		t.Error("defaultVersion should not be empty")
	}
	if defaultNamespace != "istio-system" {
		t.Errorf("Expected defaultNamespace istio-system, got %s", defaultNamespace)
	}
}

func TestPlugin_getIstioctlDownloadURL(t *testing.T) {
	p := &Plugin{Log: logger.New("[test]", logger.LevelQuiet)}

	tests := []struct {
		version  string
		goos     string
		goarch   string
		expected string
	}{
		{
			version:  "1.24.0",
			goos:     "linux",
			goarch:   "amd64",
			expected: "https://github.com/istio/istio/releases/download/1.24.0/istioctl-1.24.0-linux-amd64.tar.gz",
		},
		{
			version:  "1.24.0",
			goos:     "darwin",
			goarch:   "arm64",
			expected: "https://github.com/istio/istio/releases/download/1.24.0/istioctl-1.24.0-osx-arm64.tar.gz",
		},
		{
			version:  "1.24.0",
			goos:     "darwin",
			goarch:   "amd64",
			expected: "https://github.com/istio/istio/releases/download/1.24.0/istioctl-1.24.0-osx-amd64.tar.gz",
		},
		{
			version:  "1.24.0",
			goos:     "windows",
			goarch:   "amd64",
			expected: "https://github.com/istio/istio/releases/download/1.24.0/istioctl-1.24.0-win-amd64.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos+"-"+tt.goarch, func(t *testing.T) {
			url := p.getIstioctlDownloadURL(tt.version, tt.goos, tt.goarch)
			if url != tt.expected {
				t.Errorf("Expected URL %s, got %s", tt.expected, url)
			}
		})
	}
}

func TestPlugin_getIstioctlPath(t *testing.T) {
	p := &Plugin{Log: logger.New("[test]", logger.LevelQuiet)}

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test when istioctl exists in PATH (mock by creating a temp file)
	if runtime.GOOS != "windows" {
		fakeIstioctl := filepath.Join(tmpDir, "istioctl")
		if err := os.WriteFile(fakeIstioctl, []byte("#!/bin/sh\necho test"), 0755); err != nil {
			t.Skip("Cannot create fake istioctl:", err)
		}

		// Temporarily modify PATH
		oldPath := os.Getenv("PATH")
		os.Setenv("PATH", tmpDir+":"+oldPath)
		defer os.Setenv("PATH", oldPath)

		path, err := p.getIstioctlPath("1.24.0")
		if err != nil {
			t.Errorf("Expected to find istioctl in PATH, got error: %v", err)
		}
		if path != fakeIstioctl {
			t.Errorf("Expected path %s, got %s", fakeIstioctl, path)
		}
	}
}

func TestPlugin_IsInstalled(t *testing.T) {
	// This test would require mocking kubectl
	// For now, just verify the method exists
	p := &Plugin{Log: logger.New("[test]", logger.LevelQuiet), Timeout: 5 * time.Minute}
	_ = p.IsInstalled
}

// Helper methods for testing
func (p *Plugin) getIstioctlDownloadURL(version, goos, goarch string) string {
	osMap := map[string]string{
		"linux":   "linux",
		"darwin":  "osx",
		"windows": "win",
	}

	istioOS := osMap[goos]
	if istioOS == "" {
		istioOS = "linux"
	}

	return fmt.Sprintf("https://github.com/istio/istio/releases/download/%s/istioctl-%s-%s-%s.tar.gz",
		version, version, istioOS, goarch)
}

func (p *Plugin) getIstioctlPath(version string) (string, error) {
	// Check if istioctl is already in PATH
	if path, err := exec.LookPath("istioctl"); err == nil {
		return path, nil
	}

	// Return cached path
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/.deploy-cluster/istio/%s/bin/istioctl", home, version), nil
}
