package drift

import (
	"testing"

	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/template"
)

// TestNewDetector tests the creation of a new detector
func TestNewDetector(t *testing.T) {
	log := logger.New("[test]", logger.LevelQuiet)
	d := NewDetector(log)

	if d == nil {
		t.Fatal("NewDetector returned nil")
	}

	if d.log != log {
		t.Error("Logger not set correctly")
	}
}

// TestFormatResult_NoDrift tests formatting when there's no drift
func TestFormatResult_NoDrift(t *testing.T) {
	result := &Result{
		Changes:  []Change{},
		HasDrift: false,
	}

	output := FormatResult(result)
	expected := "✓ No drift detected. Cluster matches template."
	
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

// TestFormatResult_WithDrift tests formatting when drift exists
func TestFormatResult_WithDrift(t *testing.T) {
	result := &Result{
		Changes: []Change{
			{
				Type:     ChangeTypeRemove,
				Plugin:   "storage",
				Resource: "local-path-provisioner",
				Message:  "Storage plugin is enabled but not installed",
			},
			{
				Type:     ChangeTypeOrphan,
				Plugin:   "custom-apps",
				Resource: "orphan-app",
				Message:  "Orphan app is installed but not in template",
			},
			{
				Type:        ChangeTypeModify,
				Plugin:      "monitoring",
				Resource:    "kube-prometheus-stack",
				Property:    "version",
				ClusterVal:  "72.6.0",
				TemplateVal: "72.6.2",
				Message:     "Version drift: cluster=72.6.0, template=72.6.2",
			},
		},
		HasDrift: true,
	}

	output := FormatResult(result)

	// Check that output contains expected sections
	if output == "" {
		t.Error("Expected non-empty output")
	}

	if output == "✓ No drift detected. Cluster matches template." {
		t.Error("Should not show no-drift message when drift exists")
	}

	// Check for drift indicators
	expectedSections := []string{
		"Drift Detection Results:",
		"Missing",
		"Orphans",
		"Modified",
		"storage",
		"custom-apps",
		"monitoring",
		"Total:",
	}

	for _, section := range expectedSections {
		if !containsSubstring(output, section) {
			t.Errorf("Expected output to contain %q", section)
		}
	}
}

// TestFormatResult_OnlyMissing tests formatting with only missing resources
func TestFormatResult_OnlyMissing(t *testing.T) {
	result := &Result{
		Changes: []Change{
			{
				Type:     ChangeTypeRemove,
				Plugin:   "storage",
				Resource: "local-path-provisioner",
				Message:  "Storage plugin is enabled but not installed",
			},
		},
		HasDrift: true,
	}

	output := FormatResult(result)

	if !containsSubstring(output, "Missing") {
		t.Error("Expected output to contain 'Missing'")
	}
	if containsSubstring(output, "Orphans") {
		t.Error("Should not contain 'Orphans' section")
	}
	if containsSubstring(output, "Modified") {
		t.Error("Should not contain 'Modified' section")
	}
}

// TestFormatResult_OnlyOrphans tests formatting with only orphan resources
func TestFormatResult_OnlyOrphans(t *testing.T) {
	result := &Result{
		Changes: []Change{
			{
				Type:     ChangeTypeOrphan,
				Plugin:   "custom-apps",
				Resource: "orphan-app",
				Message:  "Orphan app is installed but not in template",
			},
		},
		HasDrift: true,
	}

	output := FormatResult(result)

	if containsSubstring(output, "Missing") {
		t.Error("Should not contain 'Missing' section")
	}
	if !containsSubstring(output, "Orphans") {
		t.Error("Expected output to contain 'Orphans'")
	}
	if containsSubstring(output, "Modified") {
		t.Error("Should not contain 'Modified' section")
	}
}

// TestFormatResult_OnlyModified tests formatting with only modified resources
func TestFormatResult_OnlyModified(t *testing.T) {
	result := &Result{
		Changes: []Change{
			{
				Type:        ChangeTypeModify,
				Plugin:      "monitoring",
				Resource:    "kube-prometheus-stack",
				Property:    "version",
				ClusterVal:  "72.6.0",
				TemplateVal: "72.6.2",
				Message:     "Version drift: cluster=72.6.0, template=72.6.2",
			},
		},
		HasDrift: true,
	}

	output := FormatResult(result)

	if containsSubstring(output, "Missing") {
		t.Error("Should not contain 'Missing' section")
	}
	if containsSubstring(output, "Orphans") {
		t.Error("Should not contain 'Orphans' section")
	}
	if !containsSubstring(output, "Modified") {
		t.Error("Expected output to contain 'Modified'")
	}
}

// TestIsSystemRelease tests the system release detection
func TestIsSystemRelease(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"kube-prometheus-stack", true},
		{"headlamp", true},
		{"external-dns", true},
		{"my-custom-app", false},
		{"redis", false},
		{"postgresql", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isSystemRelease(tt.name)
		if result != tt.expected {
			t.Errorf("isSystemRelease(%q) = %v, expected %v", tt.name, result, tt.expected)
		}
	}
}

// TestChangeTypeConstants tests the change type constants
func TestChangeTypeConstants(t *testing.T) {
	if ChangeTypeAdd != "ADD" {
		t.Errorf("Expected ChangeTypeAdd = 'ADD', got %s", ChangeTypeAdd)
	}
	if ChangeTypeRemove != "REMOVE" {
		t.Errorf("Expected ChangeTypeRemove = 'REMOVE', got %s", ChangeTypeRemove)
	}
	if ChangeTypeModify != "MODIFY" {
		t.Errorf("Expected ChangeTypeModify = 'MODIFY', got %s", ChangeTypeModify)
	}
	if ChangeTypeOrphan != "ORPHAN" {
		t.Errorf("Expected ChangeTypeOrphan = 'ORPHAN', got %s", ChangeTypeOrphan)
	}
}

// TestResultStruct tests the Result struct
func TestResultStruct(t *testing.T) {
	// Test empty result
	r1 := &Result{
		Changes:  []Change{},
		HasDrift: false,
	}
	if r1.HasDrift {
		t.Error("Empty result should not have drift")
	}

	// Test result with changes
	r2 := &Result{
		Changes: []Change{
			{Type: ChangeTypeAdd, Plugin: "test"},
		},
		HasDrift: true,
	}
	if !r2.HasDrift {
		t.Error("Result with changes should have drift")
	}
	if len(r2.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(r2.Changes))
	}
}

// TestChangeStruct tests the Change struct fields
func TestChangeStruct(t *testing.T) {
	change := Change{
		Type:        ChangeTypeModify,
		Plugin:      "monitoring",
		Resource:    "kube-prometheus-stack",
		Property:    "version",
		ClusterVal:  "72.6.0",
		TemplateVal: "72.6.2",
		Message:     "Version drift detected",
	}

	if change.Type != ChangeTypeModify {
		t.Errorf("Expected type %s, got %s", ChangeTypeModify, change.Type)
	}
	if change.Plugin != "monitoring" {
		t.Errorf("Expected plugin 'monitoring', got %s", change.Plugin)
	}
	if change.Resource != "kube-prometheus-stack" {
		t.Errorf("Expected resource 'kube-prometheus-stack', got %s", change.Resource)
	}
	if change.Property != "version" {
		t.Errorf("Expected property 'version', got %s", change.Property)
	}
	if change.ClusterVal != "72.6.0" {
		t.Errorf("Expected cluster value '72.6.0', got %s", change.ClusterVal)
	}
	if change.TemplateVal != "72.6.2" {
		t.Errorf("Expected template value '72.6.2', got %s", change.TemplateVal)
	}
	if change.Message != "Version drift detected" {
		t.Errorf("Expected message 'Version drift detected', got %s", change.Message)
	}
}

// TestDetect_EmptyTemplate tests detection with empty template
func TestDetect_EmptyTemplate(t *testing.T) {
	log := logger.New("[test]", logger.LevelQuiet)
	detector := NewDetector(log)

	tmpl := &template.Template{
		Name:    "test-cluster",
		Plugins: template.PluginsTemplate{},
	}

	// This will detect orphan resources if they exist in cluster
	_, err := detector.Detect(tmpl, "kind-test-cluster")
	// We expect this to run without error
	if err != nil {
		t.Errorf("Detect with empty template should not error: %v", err)
	}
}

// TestDetect_AllPluginsDisabled tests detection when all plugins are disabled
func TestDetect_AllPluginsDisabled(t *testing.T) {
	log := logger.New("[test]", logger.LevelQuiet)
	detector := NewDetector(log)

	tmpl := &template.Template{
		Name: "test-cluster",
		Plugins: template.PluginsTemplate{
			Storage: &template.StorageTemplate{
				Enabled: false,
				Type:    "local-path",
			},
			Ingress: &template.IngressTemplate{
				Enabled: false,
				Type:    "nginx",
			},
		},
	}

	_, err := detector.Detect(tmpl, "kind-test-cluster")
	if err != nil {
		t.Errorf("Detect with disabled plugins should not error: %v", err)
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
