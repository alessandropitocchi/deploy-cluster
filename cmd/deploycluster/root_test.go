package main

import (
	"testing"
	"time"
)

// executeCommand resets global state and runs rootCmd with the given args.
// Returns the error from Execute.
func executeCommand(args ...string) error {
	// Reset globals to defaults before each test
	verbose = false
	quiet = false
	globalTimeout = 5 * time.Minute

	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func TestRootFlags_Defaults(t *testing.T) {
	f := rootCmd.PersistentFlags()

	// Check registered defaults via flag lookup (doesn't require Execute)
	vf := f.Lookup("verbose")
	if vf == nil {
		t.Fatal("verbose flag not registered")
	}
	if vf.DefValue != "false" {
		t.Errorf("verbose default = %q, want %q", vf.DefValue, "false")
	}

	qf := f.Lookup("quiet")
	if qf == nil {
		t.Fatal("quiet flag not registered")
	}
	if qf.DefValue != "false" {
		t.Errorf("quiet default = %q, want %q", qf.DefValue, "false")
	}

	tf := f.Lookup("timeout")
	if tf == nil {
		t.Fatal("timeout flag not registered")
	}
	if tf.DefValue != "5m0s" {
		t.Errorf("timeout default = %q, want %q", tf.DefValue, "5m0s")
	}
}

func TestRootFlags_VerboseAndQuietMutuallyExclusive(t *testing.T) {
	// We need a subcommand that triggers PersistentPreRunE.
	// "status" is a good candidate since it will fail on missing config,
	// but PersistentPreRunE runs first.
	err := executeCommand("status", "--verbose", "--quiet", "--template", "nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for --verbose and --quiet together")
	}
	if got := err.Error(); got != "--verbose and --quiet are mutually exclusive" {
		t.Errorf("error = %q, want mutual exclusivity message", got)
	}
}

func TestRootFlags_TimeoutNegative(t *testing.T) {
	err := executeCommand("status", "--timeout", "-1s", "--template", "nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
	if got := err.Error(); got != "--timeout must be a positive duration" {
		t.Errorf("error = %q, want positive duration message", got)
	}
}

func TestRootFlags_TimeoutZero(t *testing.T) {
	err := executeCommand("status", "--timeout", "0s", "--template", "nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for zero timeout")
	}
	if got := err.Error(); got != "--timeout must be a positive duration" {
		t.Errorf("error = %q, want positive duration message", got)
	}
}

func TestRootFlags_TimeoutCustomValid(t *testing.T) {
	// With a valid timeout, PersistentPreRunE should pass.
	// The command will fail later (no config file), but timeout validation passes.
	err := executeCommand("status", "--timeout", "30s", "--template", "nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error (missing config), but not a timeout validation error")
	}
	// Should NOT be a timeout error
	if got := err.Error(); got == "--timeout must be a positive duration" {
		t.Error("30s should be accepted as a valid timeout")
	}
}

func TestRootFlags_TimeoutParsing(t *testing.T) {
	tests := []struct {
		flag string
		want time.Duration
	}{
		{"10m", 10 * time.Minute},
		{"30s", 30 * time.Second},
		{"1h", time.Hour},
		{"2m30s", 2*time.Minute + 30*time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			globalTimeout = 0
			rootCmd.SetArgs([]string{"status", "--timeout", tt.flag, "--template", "nonexistent.yaml"})
			_ = rootCmd.Execute()

			if globalTimeout != tt.want {
				t.Errorf("globalTimeout = %v, want %v", globalTimeout, tt.want)
			}
		})
	}
}

func TestSubcommands_Registered(t *testing.T) {
	expected := []string{"create", "upgrade", "destroy", "status", "init", "get", "uninstall"}
	commands := rootCmd.Commands()

	registered := make(map[string]bool)
	for _, cmd := range commands {
		registered[cmd.Name()] = true
	}

	for _, name := range expected {
		if !registered[name] {
			t.Errorf("subcommand %q not registered on rootCmd", name)
		}
	}
}

func TestCreateCmd_Flags(t *testing.T) {
	f := createCmd.Flags()

	// --template
	cf := f.Lookup("template")
	if cf == nil {
		t.Fatal("create should have --template flag")
	}
	if cf.DefValue != "template.yaml" {
		t.Errorf("--template default = %q, want %q", cf.DefValue, "template.yaml")
	}
	if cf.Shorthand != "t" {
		t.Errorf("--template shorthand = %q, want %q", cf.Shorthand, "t")
	}

	// --env
	ef := f.Lookup("env")
	if ef == nil {
		t.Fatal("create should have --env flag")
	}
	if ef.DefValue != ".env" {
		t.Errorf("--env default = %q, want %q", ef.DefValue, ".env")
	}

	// --fail-fast
	ff := f.Lookup("fail-fast")
	if ff == nil {
		t.Fatal("create should have --fail-fast flag")
	}
	if ff.DefValue != "false" {
		t.Errorf("--fail-fast default = %q, want %q", ff.DefValue, "false")
	}
}

func TestUpgradeCmd_Flags(t *testing.T) {
	f := upgradeCmd.Flags()

	// --template
	cf := f.Lookup("template")
	if cf == nil {
		t.Fatal("upgrade should have --template flag")
	}

	// --env
	ef := f.Lookup("env")
	if ef == nil {
		t.Fatal("upgrade should have --env flag")
	}

	// --dry-run
	dr := f.Lookup("dry-run")
	if dr == nil {
		t.Fatal("upgrade should have --dry-run flag")
	}
	if dr.DefValue != "false" {
		t.Errorf("--dry-run default = %q, want %q", dr.DefValue, "false")
	}

	// --fail-fast
	ff := f.Lookup("fail-fast")
	if ff == nil {
		t.Fatal("upgrade should have --fail-fast flag")
	}
}

func TestDestroyCmd_Flags(t *testing.T) {
	f := destroyCmd.Flags()

	cf := f.Lookup("template")
	if cf == nil {
		t.Fatal("destroy should have --template flag")
	}

	nf := f.Lookup("name")
	if nf == nil {
		t.Fatal("destroy should have --name flag")
	}
	if nf.Shorthand != "n" {
		t.Errorf("--name shorthand = %q, want %q", nf.Shorthand, "n")
	}
}

func TestStatusCmd_Flags(t *testing.T) {
	f := statusCmd.Flags()

	cf := f.Lookup("template")
	if cf == nil {
		t.Fatal("status should have --template flag")
	}
	if cf.DefValue != "template.yaml" {
		t.Errorf("--template default = %q, want %q", cf.DefValue, "template.yaml")
	}
}

func TestInitCmd_Flags(t *testing.T) {
	f := initCmd.Flags()

	of := f.Lookup("output")
	if of == nil {
		t.Fatal("init should have --output flag")
	}
	if of.DefValue != "template.yaml" {
		t.Errorf("--output default = %q, want %q", of.DefValue, "template.yaml")
	}
	if of.Shorthand != "o" {
		t.Errorf("--output shorthand = %q, want %q", of.Shorthand, "o")
	}
}

func TestGetCmd_Subcommands(t *testing.T) {
	expected := []string{"clusters", "nodes", "kubeconfig"}
	commands := getCmd.Commands()

	registered := make(map[string]bool)
	for _, cmd := range commands {
		registered[cmd.Name()] = true
	}

	for _, name := range expected {
		if !registered[name] {
			t.Errorf("get subcommand %q not registered", name)
		}
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		quiet   bool
	}{
		{"normal", false, false},
		{"verbose", true, false},
		{"quiet", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verbose = tt.verbose
			quiet = tt.quiet
			log := newLogger("[test]")
			if log == nil {
				t.Fatal("newLogger should not return nil")
			}
		})
	}
	// Reset
	verbose = false
	quiet = false
}

func TestGetProvider_Kind(t *testing.T) {
	p, err := getProvider("kind")
	if err != nil {
		t.Fatalf("getProvider(kind) error = %v", err)
	}
	if p == nil {
		t.Fatal("provider should not be nil")
	}
}

func TestGetProvider_Unknown(t *testing.T) {
	_, err := getProvider("docker-desktop")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if got := err.Error(); got != "unknown provider: docker-desktop" {
		t.Errorf("error = %q, want specific message", got)
	}
}
