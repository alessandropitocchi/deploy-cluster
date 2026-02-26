package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	verbose       bool
	quiet         bool
	globalTimeout time.Duration
	Version       = "dev" // Set at build time with -ldflags
)

var rootCmd = &cobra.Command{
	Use:   "klastr",
	Short: "Deploy Kubernetes clusters with plugins",
	Long: `klastr is a CLI tool for deploying Kubernetes clusters
on various providers (kind, k3d) with configurable topology
and plugin support (ArgoCD, storage, ingress).`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if verbose && quiet {
			return fmt.Errorf("--verbose and --quiet are mutually exclusive")
		}
		if globalTimeout <= 0 {
			return fmt.Errorf("--timeout must be a positive duration")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("klastr version %s\n", Version)
		fmt.Println("\nUsage: klastr [command]")
		fmt.Println("\nRun 'klastr --help' for more information")
	},
}

// newLogger creates a logger based on global flag state.
func newLogger(prefix string) *logger.Logger {
	level := logger.LevelNormal
	if verbose {
		level = logger.LevelVerbose
	} else if quiet {
		level = logger.LevelQuiet
	}
	return logger.New(prefix, level)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose (debug) output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress all output except errors")
	rootCmd.PersistentFlags().DurationVar(&globalTimeout, "timeout", 5*time.Minute, "timeout for plugin operations (kubectl/helm)")
}
