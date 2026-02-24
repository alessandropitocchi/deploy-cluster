package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var switchExecCommand = exec.Command

type clusterEntry struct {
	Name     string
	Provider string
	Context  string
}

var switchCmd = &cobra.Command{
	Use:   "switch [cluster-name]",
	Short: "Switch kubectl context between clusters",
	Long: `Switch the active kubectl context to a cluster.
Without arguments, lists all clusters from kind and k3d and highlights the current context.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return listClusters()
		}
		return switchToCluster(args[0])
	},
}

func listClusters() error {
	var entries []clusterEntry

	// Get kind clusters
	if _, err := exec.LookPath("kind"); err == nil {
		cmd := switchExecCommand("kind", "get", "clusters")
		output, _ := cmd.Output()
		clusters := strings.TrimSpace(string(output))
		if clusters != "" {
			for _, c := range strings.Split(clusters, "\n") {
				c = strings.TrimSpace(c)
				if c != "" {
					entries = append(entries, clusterEntry{
						Name:     c,
						Provider: "kind",
						Context:  fmt.Sprintf("kind-%s", c),
					})
				}
			}
		}
	}

	// Get k3d clusters
	if _, err := exec.LookPath("k3d"); err == nil {
		cmd := switchExecCommand("k3d", "cluster", "list", "--no-headers")
		output, _ := cmd.Output()
		clusters := strings.TrimSpace(string(output))
		if clusters != "" {
			for _, line := range strings.Split(clusters, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) > 0 {
					entries = append(entries, clusterEntry{
						Name:     fields[0],
						Provider: "k3d",
						Context:  fmt.Sprintf("k3d-%s", fields[0]),
					})
				}
			}
		}
	}

	if len(entries) == 0 {
		fmt.Println("No clusters found")
		return nil
	}

	// Get current context
	ctxCmd := switchExecCommand("kubectl", "config", "current-context")
	ctxOutput, _ := ctxCmd.Output()
	currentContext := strings.TrimSpace(string(ctxOutput))

	fmt.Println("CLUSTERS")
	fmt.Println("────────")
	for _, e := range entries {
		if e.Context == currentContext {
			fmt.Printf("● %s (%s) (current)\n", e.Name, e.Provider)
		} else {
			fmt.Printf("  %s (%s)\n", e.Name, e.Provider)
		}
	}

	return nil
}

func switchToCluster(name string) error {
	// Search in both providers
	var context string

	// Check kind
	if _, err := exec.LookPath("kind"); err == nil {
		cmd := switchExecCommand("kind", "get", "clusters")
		output, _ := cmd.Output()
		for _, c := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if strings.TrimSpace(c) == name {
				context = fmt.Sprintf("kind-%s", name)
				break
			}
		}
	}

	// Check k3d (if not found in kind)
	if context == "" {
		if _, err := exec.LookPath("k3d"); err == nil {
			cmd := switchExecCommand("k3d", "cluster", "list", "--no-headers")
			output, _ := cmd.Output()
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				fields := strings.Fields(strings.TrimSpace(line))
				if len(fields) > 0 && fields[0] == name {
					context = fmt.Sprintf("k3d-%s", name)
					break
				}
			}
		}
	}

	if context == "" {
		return fmt.Errorf("cluster '%s' not found in any provider", name)
	}

	// Switch context
	switchCmd := switchExecCommand("kubectl", "config", "use-context", context)
	if err := switchCmd.Run(); err != nil {
		return fmt.Errorf("failed to switch context: %w", err)
	}

	fmt.Printf("Switched to context '%s'\n", context)
	return nil
}

func init() {
	rootCmd.AddCommand(switchCmd)
}
