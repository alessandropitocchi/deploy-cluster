package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/config"
	"github.com/spf13/cobra"
)

var (
	destroyConfigFile string
	destroyName       string
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy a cluster",
	Long:  `Destroy an existing Kubernetes cluster and all its resources.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var clusterName string
		var providerType string

		// Get cluster name from config or flag
		if destroyName != "" {
			clusterName = destroyName
			providerType = "kind" // default
		} else {
			cfg, err := config.Load(destroyConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			clusterName = cfg.Name
			providerType = cfg.Provider.Type
		}

		// Get provider
		provider, err := getProvider(providerType)
		if err != nil {
			return err
		}

		fmt.Printf("Destroying cluster '%s'...\n", clusterName)

		if err := provider.Delete(clusterName); err != nil {
			return err
		}

		fmt.Printf("Cluster '%s' destroyed successfully!\n", clusterName)
		return nil
	},
}

func init() {
	destroyCmd.Flags().StringVarP(&destroyConfigFile, "config", "c", "cluster.yaml", "cluster configuration file")
	destroyCmd.Flags().StringVarP(&destroyName, "name", "n", "", "cluster name (overrides config)")
	rootCmd.AddCommand(destroyCmd)
}
