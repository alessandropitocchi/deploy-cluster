package main

import (
	"errors"
	"testing"

	"github.com/alepito/deploy-cluster/pkg/template"
)

func TestInstallPlugins_AllDisabled(t *testing.T) {
	cfg := &template.Template{
		Name: "test",
		Provider: template.ProviderTemplate{Type: "kind"},
	}

	results := installPlugins(cfg, "kind-test", false)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestUpgradePlugins_AllDisabled(t *testing.T) {
	cfg := &template.Template{
		Name: "test",
		Provider: template.ProviderTemplate{Type: "kind"},
	}

	results := upgradePlugins(cfg, "kind-test", false)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestHasErrors_Empty(t *testing.T) {
	if hasErrors(nil) {
		t.Error("hasErrors(nil) should be false")
	}
	if hasErrors([]pluginResult{}) {
		t.Error("hasErrors([]) should be false")
	}
}

func TestHasErrors_NoErrors(t *testing.T) {
	results := []pluginResult{
		{Name: "storage", Err: nil},
		{Name: "ingress", Err: nil},
	}
	if hasErrors(results) {
		t.Error("hasErrors should be false when all errors are nil")
	}
}

func TestHasErrors_WithError(t *testing.T) {
	results := []pluginResult{
		{Name: "storage", Err: nil},
		{Name: "ingress", Err: errors.New("failed")},
	}
	if !hasErrors(results) {
		t.Error("hasErrors should be true when an error is present")
	}
}

func TestPrintSummary_NoPanic(t *testing.T) {
	log := newLogger("")

	// Should not panic with empty results
	printSummary([]pluginResult{}, log)

	// Should not panic with mixed results
	printSummary([]pluginResult{
		{Name: "storage", Err: nil},
		{Name: "ingress", Err: errors.New("timeout")},
	}, log)
}

func TestUninstallPlugins_AllDisabled(t *testing.T) {
	cfg := &template.Template{
		Name:     "test",
		Provider: template.ProviderTemplate{Type: "kind"},
	}

	results := uninstallPlugins(cfg, "kind-test", false)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
