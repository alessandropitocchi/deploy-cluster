package main

import (
	"fmt"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	destroyTemplateFile string
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
			cfg, err := template.Load(destroyTemplateFile)
			if err != nil {
				return fmt.Errorf("failed to load template: %w", err)
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
	destroyCmd.Flags().StringVarP(&destroyTemplateFile, "template", "t", "template.yaml", "cluster template file")
	destroyCmd.Flags().StringVarP(&destroyName, "name", "n", "", "cluster name (overrides config)")
	rootCmd.AddCommand(destroyCmd)
}
