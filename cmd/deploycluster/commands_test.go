package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_MissingTemplate(t *testing.T) {
	err := executeCommand("create", "--template", "nonexistent-template-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestCreate_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(f, []byte("{{{{not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	err := executeCommand("create", "--template", f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestCreate_InvalidProvider(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "tmpl.yaml")
	if err := os.WriteFile(f, []byte(`name: test
provider:
  type: docker-desktop
cluster:
  controlPlanes: 1
  workers: 0
`), 0644); err != nil {
		t.Fatal(err)
	}

	err := executeCommand("create", "--template", f)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "not supported") && !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error = %q, want provider error", err.Error())
	}
}

func TestUpgrade_MissingTemplate(t *testing.T) {
	err := executeCommand("upgrade", "--template", "nonexistent-template-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestUpgrade_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(f, []byte("{{{{not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	err := executeCommand("upgrade", "--template", f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestDestroy_MissingTemplate(t *testing.T) {
	err := executeCommand("destroy", "--template", "nonexistent-template-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestDestroy_UnknownProvider(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "tmpl.yaml")
	if err := os.WriteFile(f, []byte(`name: test
provider:
  type: docker-desktop
cluster:
  controlPlanes: 1
  workers: 0
`), 0644); err != nil {
		t.Fatal(err)
	}

	err := executeCommand("destroy", "--template", f)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "not supported") && !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error = %q, want provider error", err.Error())
	}
}

func TestStatus_MissingTemplate(t *testing.T) {
	err := executeCommand("status", "--template", "nonexistent-template-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestInit_FileAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "template.yaml")
	if err := os.WriteFile(f, []byte("exists"), 0644); err != nil {
		t.Fatal(err)
	}

	err := executeCommand("init", "--output", f)
	if err == nil {
		t.Fatal("expected error when file already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to contain 'already exists'", err.Error())
	}
}

func TestUninstall_MissingTemplate(t *testing.T) {
	err := executeCommand("uninstall", "--template", "nonexistent-template-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestUninstall_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(f, []byte("{{{{not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	err := executeCommand("uninstall", "--template", f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to load template") {
		t.Errorf("error = %q, want it to contain 'failed to load template'", err.Error())
	}
}

func TestUninstallCmd_Flags(t *testing.T) {
	f := uninstallCmd.Flags()

	tf := f.Lookup("template")
	if tf == nil {
		t.Fatal("uninstall should have --template flag")
	}
	if tf.DefValue != "template.yaml" {
		t.Errorf("--template default = %q, want %q", tf.DefValue, "template.yaml")
	}

	ef := f.Lookup("env")
	if ef == nil {
		t.Fatal("uninstall should have --env flag")
	}

	ff := f.Lookup("fail-fast")
	if ff == nil {
		t.Fatal("uninstall should have --fail-fast flag")
	}
}
