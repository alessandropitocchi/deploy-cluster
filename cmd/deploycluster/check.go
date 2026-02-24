package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var checkExecCommand = exec.Command
var checkLookPath = exec.LookPath

type prerequisite struct {
	name       string
	versionCmd []string
	installURL string
}

var prerequisites = []prerequisite{
	{"docker", []string{"docker", "--version"}, "https://www.docker.com/"},
	{"kind", []string{"kind", "--version"}, "https://kind.sigs.k8s.io/"},
	{"k3d", []string{"k3d", "version"}, "https://k3d.io/"},
	{"kubectl", []string{"kubectl", "version", "--client", "--short"}, "https://kubernetes.io/docs/tasks/tools/"},
	{"helm", []string{"helm", "version", "--short"}, "https://helm.sh/"},
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check prerequisites",
	Long:  `Verify that all required tools are installed and show their versions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Prerequisites")
		fmt.Println("─────────────")

		allFound := true
		for _, p := range prerequisites {
			if _, err := checkLookPath(p.name); err != nil {
				fmt.Printf("✗ %-10s not found → %s\n", p.name, p.installURL)
				allFound = false
				continue
			}

			version := getVersion(p.versionCmd)
			fmt.Printf("✓ %-10s %s\n", p.name, version)
		}

		fmt.Println()
		if !allFound {
			return fmt.Errorf("some prerequisites are missing")
		}
		fmt.Println("All prerequisites satisfied!")
		return nil
	},
}

func getVersion(cmdArgs []string) string {
	cmd := checkExecCommand(cmdArgs[0], cmdArgs[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return "(unknown version)"
	}

	version := strings.TrimSpace(string(output))

	// Clean up common prefixes
	for _, prefix := range []string{
		"Docker version ",
		"kind ",
		"Client Version: ",
	} {
		if strings.HasPrefix(version, prefix) {
			version = strings.TrimPrefix(version, prefix)
			break
		}
	}

	// Take only the first line
	if idx := strings.IndexByte(version, '\n'); idx != -1 {
		version = version[:idx]
	}

	// Trim trailing commas (docker outputs "27.5.1, build ...")
	if idx := strings.Index(version, ", "); idx != -1 {
		version = version[:idx]
	}

	return version
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
