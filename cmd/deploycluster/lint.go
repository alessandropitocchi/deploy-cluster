package main

import (
	"fmt"
	"os"

	"github.com/alessandropitocchi/deploy-cluster/pkg/linter"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	lintTemplateFile string
	lintStrict       bool
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Validate template for errors and best practices",
	Long: `Validate the cluster template for errors, warnings, and best practices.

Checks include:
  - Cluster name validity (DNS subdomain format)
  - Kubernetes version format and support status
  - Cluster topology (control planes, workers)
  - Ingress host uniqueness and validity
  - Resource dependencies (e.g., ingress required for ingress hosts)
  - Best practices (storage for multi-node, monitoring, etc.)

Use --strict to treat warnings as errors.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("")

		// Load config
		log.Info("Loading template from %s...\n", lintTemplateFile)
		cfg, err := template.Load(lintTemplateFile)
		if err != nil {
			// Even if Load fails, try to lint what we have
			log.Error("Template validation failed: %v\n", err)
			os.Exit(1)
		}

		// Run linter
		l := linter.New(lintStrict)
		result := l.Lint(cfg)

		// Display results
		fmt.Println(linter.FormatResult(result))

		// Exit with error if not valid (or if strict mode and warnings exist)
		if !result.Valid {
			os.Exit(1)
		}

		if lintStrict && len(result.Issues) > 0 {
			fmt.Println("\nExiting with error due to --strict mode")
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	lintCmd.Flags().StringVarP(&lintTemplateFile, "template", "t", "template.yaml", "cluster template file")
	lintCmd.Flags().BoolVar(&lintStrict, "strict", false, "treat warnings as errors")
	rootCmd.AddCommand(lintCmd)
}
