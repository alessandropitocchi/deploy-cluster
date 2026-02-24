package customapps

import (
	"os"
	"testing"
	"time"

	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/template"
)

func testLogger() *logger.Logger {
	return logger.New("[customApps]", logger.LevelQuiet)
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
	if got := p.Name(); got != "custom-apps" {
		t.Errorf("Name() = %q, want %q", got, "custom-apps")
	}
}

func TestResolveValues_Empty(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	app := template.CustomAppTemplate{Name: "test", ChartName: "test/chart"}

	path, cleanup, err := p.resolveValues(app)
	if err != nil {
		t.Fatalf("resolveValues() error = %v", err)
	}
	if cleanup != nil {
		t.Error("cleanup should be nil for empty values")
	}
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
}

func TestResolveValues_Inline(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	app := template.CustomAppTemplate{
		Name:      "test",
		ChartName: "test/chart",
		Values: map[string]interface{}{
			"replicaCount": 3,
			"image":        "nginx",
		},
	}

	path, cleanup, err := p.resolveValues(app)
	if err != nil {
		t.Fatalf("resolveValues() error = %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup should not be nil for inline values")
	}
	defer cleanup()

	if path == "" {
		t.Fatal("path should not be empty for inline values")
	}

	// Verify file exists and contains YAML
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Error("temp values file should not be empty")
	}
}

func TestResolveValues_ValuesFile(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)

	// Create a temp values file
	tmpFile, err := os.CreateTemp("", "test-values-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	_, _ = tmpFile.WriteString("replicaCount: 5\n")
	tmpFile.Close()

	app := template.CustomAppTemplate{
		Name:       "test",
		ChartName:  "test/chart",
		ValuesFile: tmpFile.Name(),
	}

	path, cleanup, err := p.resolveValues(app)
	if err != nil {
		t.Fatalf("resolveValues() error = %v", err)
	}
	if cleanup != nil {
		t.Error("cleanup should be nil for valuesFile (no temp file created)")
	}
	if path != tmpFile.Name() {
		t.Errorf("path = %q, want %q", path, tmpFile.Name())
	}
}

func TestResolveValues_ValuesFileTakesPrecedence(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)

	tmpFile, err := os.CreateTemp("", "test-values-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	app := template.CustomAppTemplate{
		Name:       "test",
		ChartName:  "test/chart",
		ValuesFile: tmpFile.Name(),
		Values:     map[string]interface{}{"key": "value"},
	}

	path, _, err := p.resolveValues(app)
	if err != nil {
		t.Fatalf("resolveValues() error = %v", err)
	}
	// ValuesFile should win over inline Values
	if path != tmpFile.Name() {
		t.Errorf("path = %q, want %q (valuesFile should take precedence)", path, tmpFile.Name())
	}
}
