package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/alepito/deploy-cluster/pkg/template"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	initOutput string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate an initial cluster template file",
	Long: `Generate a template.yaml file through an interactive wizard
that lets you choose which plugins to enable and configure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if file already exists
		if _, err := os.Stat(initOutput); err == nil {
			return fmt.Errorf("file %s already exists, use --output to specify a different name", initOutput)
		}

		cfg, err := runInitWizard()
		if err != nil {
			return err
		}

		if err := cfg.Save(initOutput); err != nil {
			return fmt.Errorf("failed to write template: %w", err)
		}

		fmt.Printf("\nCreated %s\n", initOutput)
		fmt.Println("\nEdit the file to customize your cluster, then run:")
		fmt.Printf("  deploy-cluster create --template %s\n", initOutput)
		return nil
	},
}

func runInitWizard() (*template.Template, error) {
	// Form variables with defaults
	var (
		providerType  = "kind"
		name          = "my-cluster"
		version       = "v1.31.0"
		controlPlanes = "1"
		workers       = "2"
		ingressType   = "nginx"
		plugins       []string

		// Ingress hosts
		monitoringHost = "grafana.localhost"
		dashboardHost  = "headlamp.localhost"
		argocdHost     = "argocd.localhost"

		// ArgoCD
		argocdNamespace = "argocd"
		argocdVersion   = "stable"
	)

	// Helper to check if a plugin is selected
	hasPlugin := func(name string) bool {
		for _, p := range plugins {
			if p == name {
				return true
			}
		}
		return false
	}

	form := huh.NewForm(
		// Group 1: Provider selection
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Cluster provider").
				Options(
					huh.NewOption("kind", "kind"),
					huh.NewOption("k3d", "k3d"),
				).
				Value(&providerType),
		).Title("Provider"),

		// Group 2: Cluster basics
		huh.NewGroup(
			huh.NewInput().
				Title("Cluster name").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),

			huh.NewInput().
				Title("Kubernetes version").
				Value(&version),

			huh.NewInput().
				Title("Control plane nodes").
				Value(&controlPlanes).
				Validate(func(s string) error {
					n, err := strconv.Atoi(s)
					if err != nil || n < 1 {
						return fmt.Errorf("must be at least 1")
					}
					return nil
				}),

			huh.NewInput().
				Title("Worker nodes").
				Value(&workers).
				Validate(func(s string) error {
					n, err := strconv.Atoi(s)
					if err != nil || n < 0 {
						return fmt.Errorf("must be 0 or more")
					}
					return nil
				}),
		).Title("Cluster Configuration"),

		// Group 3: Plugin selection
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which plugins do you want to enable?").
				Options(
					huh.NewOption("Storage (local-path-provisioner)", "storage"),
					huh.NewOption("Ingress", "ingress"),
					huh.NewOption("Cert-Manager", "certmanager"),
					huh.NewOption("Monitoring (Prometheus + Grafana)", "monitoring"),
					huh.NewOption("Dashboard (Headlamp)", "dashboard"),
					huh.NewOption("ArgoCD", "argocd"),
				).
				Value(&plugins),
		).Title("Plugins"),

		// Group: Ingress type selection (conditional: ingress selected)
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Ingress controller type").
				Options(
					huh.NewOption("nginx", "nginx"),
					huh.NewOption("traefik", "traefik"),
				).
				Value(&ingressType),
		).Title("Ingress Type").WithHideFunc(func() bool {
			return !hasPlugin("ingress")
		}),

		// Group 3: Ingress hosts (conditional: ingress + target plugin selected)
		huh.NewGroup(
			huh.NewInput().
				Title("Grafana hostname").
				Value(&monitoringHost),
		).Title("Monitoring Ingress").WithHideFunc(func() bool {
			return !hasPlugin("ingress") || !hasPlugin("monitoring")
		}),

		huh.NewGroup(
			huh.NewInput().
				Title("Headlamp hostname").
				Value(&dashboardHost),
		).Title("Dashboard Ingress").WithHideFunc(func() bool {
			return !hasPlugin("ingress") || !hasPlugin("dashboard")
		}),

		huh.NewGroup(
			huh.NewInput().
				Title("ArgoCD hostname").
				Value(&argocdHost),
		).Title("ArgoCD Ingress").WithHideFunc(func() bool {
			return !hasPlugin("ingress") || !hasPlugin("argocd")
		}),

		// Group 4: ArgoCD config (conditional)
		huh.NewGroup(
			huh.NewInput().
				Title("ArgoCD namespace").
				Value(&argocdNamespace),
			huh.NewInput().
				Title("ArgoCD version").
				Value(&argocdVersion),
		).Title("ArgoCD Configuration").WithHideFunc(func() bool {
			return !hasPlugin("argocd")
		}),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Build config from form values
	cp, _ := strconv.Atoi(controlPlanes)
	w, _ := strconv.Atoi(workers)

	cfg := &template.Template{
		Name: name,
		Provider: template.ProviderTemplate{
			Type: providerType,
		},
		Cluster: template.ClusterTemplate{
			ControlPlanes: cp,
			Workers:       w,
			Version:       version,
		},
	}

	hasIngress := hasPlugin("ingress")

	if hasPlugin("storage") {
		cfg.Plugins.Storage = &template.StorageTemplate{
			Enabled: true,
			Type:    "local-path",
		}
	}

	if hasIngress {
		cfg.Plugins.Ingress = &template.IngressTemplate{
			Enabled: true,
			Type:    ingressType,
		}
	}

	if hasPlugin("certmanager") {
		cfg.Plugins.CertManager = &template.CertManagerTemplate{
			Enabled: true,
			Version: "v1.16.3",
		}
	}

	if hasPlugin("monitoring") {
		mon := &template.MonitoringTemplate{
			Enabled: true,
			Type:    "prometheus",
		}
		if hasIngress {
			mon.Ingress = &template.MonitoringIngressTemplate{
				Enabled: true,
				Host:    monitoringHost,
			}
		}
		cfg.Plugins.Monitoring = mon
	}

	if hasPlugin("dashboard") {
		dash := &template.DashboardTemplate{
			Enabled: true,
			Type:    "headlamp",
		}
		if hasIngress {
			dash.Ingress = &template.DashboardIngressTemplate{
				Enabled: true,
				Host:    dashboardHost,
			}
		}
		cfg.Plugins.Dashboard = dash
	}

	if hasPlugin("argocd") {
		argo := &template.ArgoCDTemplate{
			Enabled:   true,
			Namespace: argocdNamespace,
			Version:   argocdVersion,
		}
		if hasIngress {
			argo.Ingress = &template.ArgoCDIngressTemplate{
				Enabled: true,
				Host:    argocdHost,
			}
		}
		cfg.Plugins.ArgoCD = argo
	}

	return cfg, nil
}

func init() {
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "template.yaml", "output file path")
	rootCmd.AddCommand(initCmd)
}
