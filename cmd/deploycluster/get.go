package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	getProviderFlag     string
	nodesProviderFlag   string
	kubecfgProviderFlag string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get cluster information",
	Long:  `Display information about existing clusters.`,
}

var getClustersCmd = &cobra.Command{
	Use:   "clusters",
	Short: "List all clusters",
	Long:  `List all existing clusters from kind and/or k3d.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		found := false

		if getProviderFlag == "" || getProviderFlag == "kind" {
			if kindFound, err := listProviderClusters("kind"); err == nil && kindFound {
				found = true
			}
		}

		if getProviderFlag == "" || getProviderFlag == "k3d" {
			if k3dFound, err := listProviderClusters("k3d"); err == nil && k3dFound {
				found = true
			}
		}

		if !found {
			fmt.Println("No clusters found")
		}

		return nil
	},
}

func listProviderClusters(providerType string) (bool, error) {
	switch providerType {
	case "kind":
		if _, err := exec.LookPath("kind"); err != nil {
			return false, nil // skip if not installed
		}
		kindCmd := exec.Command("kind", "get", "clusters")
		output, err := kindCmd.Output()
		if err != nil {
			return false, err
		}
		clusters := strings.TrimSpace(string(output))
		if clusters == "" {
			return false, nil
		}
		fmt.Println("KIND CLUSTERS")
		fmt.Println("─────────────")
		for _, cluster := range strings.Split(clusters, "\n") {
			if cluster != "" {
				fmt.Printf("  %s\n", cluster)
			}
		}
		fmt.Println()
		return true, nil

	case "k3d":
		if _, err := exec.LookPath("k3d"); err != nil {
			return false, nil // skip if not installed
		}
		k3dCmd := exec.Command("k3d", "cluster", "list", "--no-headers")
		output, err := k3dCmd.Output()
		if err != nil {
			return false, err
		}
		clusters := strings.TrimSpace(string(output))
		if clusters == "" {
			return false, nil
		}
		fmt.Println("K3D CLUSTERS")
		fmt.Println("────────────")
		for _, line := range strings.Split(clusters, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// k3d list output: NAME SERVERS AGENTS ...
			fields := strings.Fields(line)
			if len(fields) > 0 {
				fmt.Printf("  %s\n", fields[0])
			}
		}
		fmt.Println()
		return true, nil
	}
	return false, nil
}

var getNodesCmd = &cobra.Command{
	Use:   "nodes [cluster-name]",
	Short: "List nodes in a cluster",
	Long:  `List all nodes in a specific cluster.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterName := "kind"
		if len(args) > 0 {
			clusterName = args[0]
		}

		switch nodesProviderFlag {
		case "k3d":
			dockerCmd := exec.Command("docker", "ps",
				"--filter", "label=app=k3d",
				"--filter", fmt.Sprintf("label=k3d.cluster=%s", clusterName),
				"--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}")
			output, err := dockerCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to list nodes: %w", err)
			}
			result := strings.TrimSpace(string(output))
			if result == "" || result == "NAMES\tSTATUS\tPORTS" {
				fmt.Printf("No nodes found for cluster '%s'\n", clusterName)
				return nil
			}
			fmt.Printf("NODES FOR K3D CLUSTER '%s'\n", clusterName)
			fmt.Println("──────────────────────────────────────────────────────────────")
			fmt.Println(result)

		default: // kind
			dockerCmd := exec.Command("docker", "ps",
				"--filter", fmt.Sprintf("label=io.x-k8s.kind.cluster=%s", clusterName),
				"--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}")
			output, err := dockerCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to list nodes: %w", err)
			}
			result := strings.TrimSpace(string(output))
			if result == "" || result == "NAMES\tSTATUS\tPORTS" {
				fmt.Printf("No nodes found for cluster '%s'\n", clusterName)
				return nil
			}
			fmt.Printf("NODES FOR CLUSTER '%s'\n", clusterName)
			fmt.Println("──────────────────────────────────────────────────────────────")
			fmt.Println(result)
		}

		return nil
	},
}

var getKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [cluster-name]",
	Short: "Get kubeconfig for a cluster",
	Long:  `Output the kubeconfig for a specific cluster.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterName := "kind"
		if len(args) > 0 {
			clusterName = args[0]
		}

		switch kubecfgProviderFlag {
		case "k3d":
			k3dCmd := exec.Command("k3d", "kubeconfig", "get", clusterName)
			output, err := k3dCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get kubeconfig: %w", err)
			}
			fmt.Print(string(output))

		default: // kind
			kindCmd := exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
			output, err := kindCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get kubeconfig: %w", err)
			}
			fmt.Print(string(output))
		}

		return nil
	},
}

func init() {
	getClustersCmd.Flags().StringVar(&getProviderFlag, "provider", "", "filter by provider (kind, k3d)")
	getNodesCmd.Flags().StringVar(&nodesProviderFlag, "provider", "kind", "cluster provider (kind, k3d)")
	getKubeconfigCmd.Flags().StringVar(&kubecfgProviderFlag, "provider", "kind", "cluster provider (kind, k3d)")

	getCmd.AddCommand(getClustersCmd)
	getCmd.AddCommand(getNodesCmd)
	getCmd.AddCommand(getKubeconfigCmd)
	rootCmd.AddCommand(getCmd)
}
