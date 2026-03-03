package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	initOutput   string
	initDir      bool
	initProvider string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter cluster template or directory structure",
	Long: `Generate a template.yaml file or directory structure with all available options documented.

The generated template includes comprehensive comments explaining each option,
with sensible defaults to get you started quickly. Edit the file to customize
your cluster configuration.

Use --dir to generate a directory structure instead of a single file. This
allows you to organize your configuration across multiple files for better
maintainability.`,
	Example: `  # Generate template.yaml in current directory
  klastr init

  # Generate with custom name
  klastr init --output my-cluster.yaml

  # Generate directory structure
  klastr init --dir --output my-cluster/

  # Generate directory for existing cluster
  klastr init --dir --provider existing --output my-eks-cluster/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if initDir {
			return initDirectory()
		}
		return initSingleFile()
	},
}

func init() {
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "template.yaml", "output file or directory path")
	initCmd.Flags().BoolVarP(&initDir, "dir", "d", false, "generate directory structure instead of single file")
	initCmd.Flags().StringVarP(&initProvider, "provider", "p", "kind", "provider type (kind, k3d, existing)")
	rootCmd.AddCommand(initCmd)
}

// initSingleFile generates a single template.yaml file
func initSingleFile() error {
	// Check if file already exists
	if _, err := os.Stat(initOutput); err == nil {
		return fmt.Errorf("file %s already exists, use --output to specify a different name", initOutput)
	}

	// Generate commented template with defaults
	content := generateStarterTemplate()
	if err := os.WriteFile(initOutput, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}

	fmt.Printf("✓ Created %s\n", initOutput)
	fmt.Println("\nNext steps:")
	fmt.Printf("  1. Edit %s to customize your cluster\n", initOutput)
	fmt.Printf("  2. Validate: klastr lint --template %s\n", initOutput)
	fmt.Printf("  3. Run:      klastr run --template %s\n", initOutput)
	return nil
}

// initDirectory generates a directory structure with multiple config files
func initDirectory() error {
	// Check if directory already exists
	if _, err := os.Stat(initOutput); err == nil {
		return fmt.Errorf("directory %s already exists, use --output to specify a different name", initOutput)
	}

	// Create directory structure
	dirs := []string{
		initOutput,
		filepath.Join(initOutput, "plugins"),
		filepath.Join(initOutput, "apps"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Generate main config file
	mainConfig := generateMainConfig()
	mainPath := filepath.Join(initOutput, "klastr.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		return fmt.Errorf("failed to write main config: %w", err)
	}

	// Generate plugin configs
	if err := generatePluginConfigs(); err != nil {
		return err
	}

	// Generate README
	readme := generateDirREADME()
	readmePath := filepath.Join(initOutput, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	// Generate .env.example
	envExample := generateEnvExample()
	envPath := filepath.Join(initOutput, ".env.example")
	if err := os.WriteFile(envPath, []byte(envExample), 0644); err != nil {
		return fmt.Errorf("failed to write .env.example: %w", err)
	}

	fmt.Printf("✓ Created directory structure in %s/\n", initOutput)
	fmt.Println("\nGenerated files:")
	fmt.Printf("  %s/klastr.yaml      # Main configuration\n", initOutput)
	fmt.Printf("  %s/plugins/         # Plugin configurations\n", initOutput)
	fmt.Printf("  %s/apps/            # Custom application configs\n", initOutput)
	fmt.Printf("  %s/.env.example     # Environment variables template\n", initOutput)
	fmt.Printf("  %s/README.md        # Documentation\n", initOutput)
	fmt.Println("\nNext steps:")
	fmt.Printf("  1. Edit %s/klastr.yaml to customize your cluster\n", initOutput)
	fmt.Printf("  2. Enable plugins by editing files in %s/plugins/\n", initOutput)
	fmt.Printf("  3. Validate:  klastr lint --template %s/\n", initOutput)
	fmt.Printf("  4. Run:       klastr run --template %s/\n", initOutput)
	return nil
}

// generateStarterTemplate creates a comprehensive starter template with all options documented
func generateStarterTemplate() string {
	return `# ╔══════════════════════════════════════════════════════════════════════════════╗
# ║                     klastr Configuration Template                    ║
# ╚══════════════════════════════════════════════════════════════════════════════╝
#
# This is a starter template with all available options documented.
# All plugins are commented out - uncomment the ones you want to enable.
#
# Quick Start:
#   1. Edit this file to customize your cluster
#   2. Validate:  klastr lint --template template.yaml
#   3. Run:       klastr run --template template.yaml
#   4. Upgrade:   klastr upgrade --template template.yaml
#
# Documentation: https://github.com/alessandropitocchi/klastr#readme
#

# ═══════════════════════════════════════════════════════════════════════════════
# Cluster Metadata
# ═══════════════════════════════════════════════════════════════════════════════

name: my-cluster  # Cluster name (used for kubecontext)

# Provider: kind, k3d, or existing
#   - kind:     Local clusters in Docker (requires kind)
#   - k3d:      Lightweight k3s clusters (requires k3d)
#   - existing: Use an existing cluster (EKS, GKE, AKS, etc.)
provider:
  type: kind
  # kubeconfig: ~/.kube/config  # Optional: for 'existing' provider
  # context: production         # Optional: kubectl context for 'existing' provider

# ═══════════════════════════════════════════════════════════════════════════════
# Cluster Topology (ignored for 'existing' provider)
# ═══════════════════════════════════════════════════════════════════════════════

cluster:
  controlPlanes: 1  # Number of control plane nodes (odd numbers recommended: 1, 3, 5)
  workers: 2        # Number of worker nodes (0 = workloads run on control planes)
  version: "v1.31.0"  # Kubernetes version (e.g., v1.31.0, v1.30.0)

# ═══════════════════════════════════════════════════════════════════════════════
# Plugins Configuration
# ═══════════════════════════════════════════════════════════════════════════════
# Uncomment the plugins you want to enable and customize their settings

plugins:

  # ─────────────────────────────────────────────────────────────────────────────
  # Storage: Dynamic volume provisioning
  # Required for: PVCs, databases, stateful applications
  # ─────────────────────────────────────────────────────────────────────────────
  # storage:
  #   enabled: true
  #   type: local-path  # Only option currently available

  # ─────────────────────────────────────────────────────────────────────────────
  # Ingress: HTTP/HTTPS routing and load balancing with Gateway API support
  # Required for: Exposing services via hostnames
  # ─────────────────────────────────────────────────────────────────────────────
  # ingress:
  #   enabled: true
  #   type: traefik              # Options: traefik, nginx-gateway-fabric
  #   # Traefik: Modern reverse proxy with native Gateway API support
  #   # nginx-gateway-fabric: F5's Gateway API implementation using NGINX

  # ─────────────────────────────────────────────────────────────────────────────
  # Cert-Manager: Automatic TLS certificate management
  # Required for: HTTPS certificates, Let's Encrypt integration
  # ─────────────────────────────────────────────────────────────────────────────
  # certManager:
  #   enabled: true
  #   version: v1.16.3  # Cert-manager version

  # ─────────────────────────────────────────────────────────────────────────────
  # External DNS: Automatic DNS record management
  # Required for: Automatic DNS updates when services change
  # ─────────────────────────────────────────────────────────────────────────────
  # externalDNS:
  #   enabled: true
  #   provider: cloudflare  # Options: cloudflare, route53, google, azure, digitalocean
  #   zone: example.com     # DNS zone to manage
  #   credentials:
  #     apiToken: ${CF_API_TOKEN}  # Use env var - see bottom of file

  # ─────────────────────────────────────────────────────────────────────────────
  # Istio: Service mesh with mTLS, traffic management, and observability
  # Features: Mutual TLS, traffic routing, canary deployments
  # ─────────────────────────────────────────────────────────────────────────────
  # istio:
  #   enabled: true
  #   profile: default  # Options: default, demo, minimal, ambient
  #   ingressGateway: true  # Deploy ingress gateway
  #   version: "1.24.0"

  # ─────────────────────────────────────────────────────────────────────────────
  # Monitoring: Prometheus + Grafana stack
  # Includes: Metrics collection, dashboards, alerting
  # ─────────────────────────────────────────────────────────────────────────────
  # monitoring:
  #   enabled: true
  #   type: prometheus
  #   version: "72.6.2"  # kube-prometheus-stack chart version
  #   # Optional: expose Grafana via ingress
  #   # ingress:
  #   #   enabled: true
  #   #   host: grafana.localhost
  #   #   # Default credentials: admin / prom-operator

  # ─────────────────────────────────────────────────────────────────────────────
  # Dashboard: Headlamp - Modern Kubernetes Web UI
  # Features: Pod logs, resource editing, cluster overview
  # ─────────────────────────────────────────────────────────────────────────────
  # dashboard:
  #   enabled: true
  #   type: headlamp
  #   version: "0.25.0"
  #   # Optional: expose Headlamp via ingress
  #   # ingress:
  #   #   enabled: true
  #   #   host: headlamp.localhost
  #   #   # Get token: kubectl create token headlamp -n headlamp

  # ─────────────────────────────────────────────────────────────────────────────
  # ArgoCD: GitOps continuous delivery
  # Features: Declarative app deployment, auto-sync, rollback
  # ─────────────────────────────────────────────────────────────────────────────
  # argocd:
  #   enabled: true
  #   namespace: argocd
  #   version: stable  # Or specific version like "v2.12.0"
  #   # Optional: expose ArgoCD via ingress
  #   # ingress:
  #   #   enabled: true
  #   #   host: argocd.localhost
  #   #   # Get password: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
  #   #
  #   # Git repositories to configure
  #   # repos:
  #   #   - name: my-repo
  #   #     url: https://github.com/myorg/myrepo
  #   #     type: git
  #   #     # For private repos, use one of:
  #   #     # sshKeyFile: ~/.ssh/id_ed25519
  #   #     # sshKeyEnv: GIT_SSH_KEY
  #   #     # insecureIgnoreHostKey: true  # Skip SSH host key verification (auto-enabled for SSH URLs)
  #   #
  #   # Applications to create
  #   # apps:
  #   #   - name: my-app
  #   #     repo: my-repo
  #   #     path: k8s/overlays/dev
  #   #     namespace: default
  #   #     targetRevision: main  # Git branch/tag

  # ─────────────────────────────────────────────────────────────────────────────
  # Custom Apps: Deploy your own Helm charts
  # Use for: Your applications, third-party charts
  # ─────────────────────────────────────────────────────────────────────────────
  # customApps:
  #   - name: my-app
  #     chartName: ./charts/my-app  # Local path
  #     # chartName: oci://registry-1.docker.io/bitnamicharts/nginx  # OCI registry
  #     # chartName: nginx  # From chart repo (requires chartRepo)
  #     # chartRepo: https://charts.bitnami.com/bitnami
  #     namespace: default
  #     version: "1.0.0"  # Chart version (optional)
  #     values:           # Inline Helm values (optional)
  #       replicaCount: 2
  #       service:
  #         type: ClusterIP
  #     # valuesFile: values.yaml  # External values file (optional, takes precedence)
  #     # Optional: expose via ingress
  #     # ingress:
  #     #   enabled: true
  #     #   host: myapp.localhost

  # ─────────────────────────────────────────────────────────────────────────────
  # S3 Snapshot Storage: Backup snapshots to S3
  # Use for: Disaster recovery, cross-region backup, long-term retention
  # ─────────────────────────────────────────────────────────────────────────────
  # snapshot:
  #   enabled: true
  #   bucket: my-k8s-backups           # S3 bucket name
  #   prefix: clusters/my-cluster/     # Optional: key prefix
  #   region: us-east-1                # Optional: AWS region (defaults to AWS_REGION)
  #   # endpoint: http://localhost:9000  # Optional: for MinIO/S3-compatible
  #   #
  #   # AWS credentials can be provided via:
  #   # - Environment variables: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
  #   # - IAM roles (when running on AWS)
  #   # - ~/.aws/credentials file

# ═══════════════════════════════════════════════════════════════════════════════
# Environment Variables & Secrets
# ═══════════════════════════════════════════════════════════════════════════════
#
# Create a .env file in the same directory as this template for secrets.
# DO NOT commit the .env file to git!
#
# Example .env file:
#   # Cloudflare API token for External DNS
#   CF_API_TOKEN=your-cloudflare-token
#
#   # AWS credentials for Route53
#   AWS_ACCESS_KEY_ID=your-aws-key
#   AWS_SECRET_ACCESS_KEY=your-aws-secret
#
#   # Git SSH key for ArgoCD private repos
#   GIT_SSH_KEY=-----BEGIN OPENSSH PRIVATE KEY-----
#
# Reference env vars in this template with ${VAR_NAME} syntax
`
}


// generateMainConfig creates the main klastr.yaml file for directory structure
func generateMainConfig() string {
	providerComment := ""
	clusterConfig := `
cluster:
  controlPlanes: 1  # Number of control plane nodes
  workers: 2        # Number of worker nodes
  version: "v1.31.0"  # Kubernetes version`

	switch initProvider {
	case "existing":
		providerComment = `# Provider: existing (EKS, GKE, AKS, etc.)
# Uses an existing Kubernetes cluster. Configure kubeconfig and context below.`
		clusterConfig = ""
	case "k3d":
		providerComment = `# Provider: k3d (lightweight k3s clusters in Docker)
# Requires k3d to be installed: https://k3d.io`
	default: // kind
		providerComment = `# Provider: kind (Kubernetes in Docker)
# Requires kind to be installed: https://kind.sigs.k8s.io`
	}

	result := fmt.Sprintf(`# ╔══════════════════════════════════════════════════════════════════════════════╗
# ║                     klastr Main Configuration                         ║
# ╚══════════════════════════════════════════════════════════════════════════════╝
#
# This is the main configuration file for your cluster.
# Additional configuration is loaded from the plugins/ and apps/ directories.
#
# Quick Start:
#   1. Edit this file to set your cluster name and provider
#   2. Enable plugins by editing files in plugins/
#   3. Add custom apps in apps/
#   4. Validate:  klastr lint --template ./
#   5. Run:       klastr run --template ./
#

# Cluster name (used for kubecontext)
name: my-cluster

%s
provider:
  type: %s
`, providerComment, initProvider)

	if initProvider == "existing" {
		result += `  # kubeconfig: ~/.kube/config  # Path to kubeconfig file
  # context: my-cluster         # Kubectl context to use
`
	}

	result += clusterConfig
	result += `

# Note: Plugin configurations are loaded from the plugins/ directory.
# See plugins/*.yaml for available plugin options.
`
	return result
}

// generatePluginConfigs creates plugin configuration files
func generatePluginConfigs() error {
	plugins := []struct {
		name    string
		content string
	}{
		{
			name: "storage.yaml",
			content: `# Storage Plugin Configuration
# Provides dynamic volume provisioning for PVCs

plugins:
  storage:
    enabled: false  # Set to true to enable
    type: local-path  # Storage provider (local-path)
`,
		},
		{
			name: "ingress.yaml",
			content: `# Ingress Controller Configuration
# HTTP/HTTPS routing and load balancing with Gateway API support

plugins:
  ingress:
    enabled: false  # Set to true to enable
    type: traefik              # Options: traefik, nginx-gateway-fabric
    # Traefik: Modern reverse proxy with native Gateway API support
    # nginx-gateway-fabric: F5's Gateway API implementation using NGINX
`,
		},
		{
			name: "cert-manager.yaml",
			content: `# Cert-Manager Configuration
# Automatic TLS certificate management

plugins:
  certManager:
    enabled: false  # Set to true to enable
    version: v1.16.3  # Cert-manager version
`,
		},
		{
			name: "external-dns.yaml",
			content: `# External DNS Configuration
# Automatic DNS record management
# Supports: cloudflare, route53, google, azure, digitalocean

plugins:
  externalDNS:
    enabled: false  # Set to true to enable
    version: "1.15.0"
    provider: cloudflare  # DNS provider
    zone: example.com     # DNS zone to manage
    # credentials:        # Provider-specific credentials
    #   apiToken: ${CF_API_TOKEN}  # Use env var reference
`,
		},
		{
			name: "istio.yaml",
			content: `# Istio Service Mesh Configuration
# mTLS, traffic management, and observability

plugins:
  istio:
    enabled: false  # Set to true to enable
    profile: default  # Options: default, demo, minimal, ambient
    version: "1.24.0"
    ingressGateway: true  # Deploy ingress gateway
`,
		},
		{
			name: "monitoring.yaml",
			content: `# Monitoring Stack Configuration
# Prometheus + Grafana for observability

plugins:
  monitoring:
    enabled: false  # Set to true to enable
    type: prometheus
    version: "72.6.2"  # kube-prometheus-stack chart version
    # Optional: expose Grafana via ingress
    # ingress:
    #   enabled: true
    #   host: grafana.localhost
    #   # Default credentials: admin / prom-operator
`,
		},
		{
			name: "dashboard.yaml",
			content: `# Kubernetes Dashboard Configuration
# Headlamp - Modern Kubernetes Web UI

plugins:
  dashboard:
    enabled: false  # Set to true to enable
    type: headlamp
    version: "0.25.0"
    # Optional: expose Headlamp via ingress
    # ingress:
    #   enabled: true
    #   host: headlamp.localhost
    #   # Get token: kubectl create token headlamp -n headlamp
`,
		},
		{
			name: "argocd.yaml",
			content: `# ArgoCD GitOps Configuration
# Declarative continuous delivery

plugins:
  argocd:
    enabled: false  # Set to true to enable
    namespace: argocd
    version: v2.13.0
    # Optional: expose ArgoCD via ingress
    # ingress:
    #   enabled: true
    #   host: argocd.localhost
    #   tls: false
    # repos:  # Git repositories to configure
    #   - name: my-repo
    #     url: git@github.com:myorg/myrepo.git
    #     sshKeyFile: ~/.ssh/id_ed25519
    #     # insecureIgnoreHostKey: true  # Skip SSH host key verification (auto-enabled for SSH)
    # apps: []   # Applications to create
`,
		},
	}

	for _, p := range plugins {
		path := filepath.Join(initOutput, "plugins", p.name)
		if err := os.WriteFile(path, []byte(p.content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", p.name, err)
		}
	}

	return nil
}

// generateDirREADME creates a README for the directory structure
func generateDirREADME() string {
	return "# klastr Cluster Configuration\n\n" +
		"This directory contains a klastr cluster configuration organized across multiple files.\n\n" +
		"## Directory Structure\n\n" +
		"```\n" +
		".\n" +
		"├── klastr.yaml          # Main configuration (cluster name, provider, topology)\n" +
		"├── plugins/             # Plugin configurations\n" +
		"│   ├── storage.yaml\n" +
		"│   ├── ingress.yaml\n" +
		"│   ├── cert-manager.yaml\n" +
		"│   ├── external-dns.yaml\n" +
		"│   ├── istio.yaml\n" +
		"│   ├── monitoring.yaml\n" +
		"│   ├── dashboard.yaml\n" +
		"│   └── argocd.yaml\n" +
		"├── apps/                # Custom application configurations\n" +
		"├── .env.example         # Environment variables template\n" +
		"└── README.md            # This file\n" +
		"```\n\n" +
		"## Usage\n\n" +
		"1. Edit main configuration:\n" +
		"   ```bash\n" +
		"   vim klastr.yaml\n" +
		"   ```\n\n" +
		"2. Enable plugins:\n" +
		"   ```bash\n" +
		"   # Edit the plugin files you need and set enabled: true\n" +
		"   vim plugins/storage.yaml\n" +
		"   vim plugins/ingress.yaml\n" +
		"   # ... etc\n" +
		"   ```\n\n" +
		"3. Configure environment variables (optional):\n" +
		"   ```bash\n" +
		"   cp .env.example .env\n" +
		"   vim .env  # Add your secrets\n" +
		"   ```\n\n" +
		"4. Validate configuration:\n" +
		"   ```bash\n" +
		"   klastr lint --template ./\n" +
		"   ```\n\n" +
		"5. Deploy cluster:\n" +
		"   ```bash\n" +
		"   klastr run --template ./\n" +
		"   ```\n\n" +
		"## Configuration Merging\n\n" +
		"When using a directory, klastr loads and merges files in this order:\n\n" +
		"1. klastr.yaml - Main configuration\n" +
		"2. plugins/*.yaml - Plugin configurations (alphabetical order)\n" +
		"3. apps/*.yaml - Custom application configurations (alphabetical order)\n\n" +
		"Later files override earlier ones for simple fields. Plugin lists are additive.\n\n" +
		"## Environment Variables\n\n" +
		"You can use environment variables in any YAML file:\n\n" +
		"```yaml\n" +
		"plugins:\n" +
		"  externalDNS:\n" +
		"    credentials:\n" +
		"      apiToken: ${CF_API_TOKEN}\n" +
		"```\n\n" +
		"Create a .env file in this directory to set variables:\n\n" +
		"```bash\n" +
		"CF_API_TOKEN=your-cloudflare-token\n" +
		"```\n\n" +
		"## Documentation\n\n" +
		"For more information, visit: https://github.com/alessandropitocchi/klastr\n"
}

// generateEnvExample creates a .env.example file
func generateEnvExample() string {
	return `# Environment Variables for klastr
#
# Copy this file to .env and fill in your secrets.
# DO NOT commit .env to git!
#
# Reference these variables in your YAML configs with ${VAR_NAME} syntax.

# ═══════════════════════════════════════════════════════════════════════════════
# External DNS Providers
# ═══════════════════════════════════════════════════════════════════════════════

# Cloudflare
# CF_API_TOKEN=your-cloudflare-api-token
# CF_API_EMAIL=your-email@example.com  # Only needed if using API key instead of token

# AWS Route53
# AWS_ACCESS_KEY_ID=your-aws-access-key
# AWS_SECRET_ACCESS_KEY=your-aws-secret-key
# AWS_REGION=us-east-1

# Google Cloud DNS
# GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json

# Azure DNS
# AZURE_CLIENT_ID=your-client-id
# AZURE_CLIENT_SECRET=your-client-secret
# AZURE_SUBSCRIPTION_ID=your-subscription-id
# AZURE_TENANT_ID=your-tenant-id
# AZURE_RESOURCE_GROUP=your-dns-resource-group

# DigitalOcean
# DO_TOKEN=your-digitalocean-token

# ═══════════════════════════════════════════════════════════════════════════════
# ArgoCD Private Repositories
# ═══════════════════════════════════════════════════════════════════════════════

# Git SSH private key (replace newlines with actual newlines)
# GIT_SSH_KEY="-----BEGIN OPENSSH PRIVATE KEY-----
# ...
# -----END OPENSSH PRIVATE KEY-----"

# Or use a file path in your YAML instead of env var

# ═══════════════════════════════════════════════════════════════════════════════
# S3 Snapshot Storage
# ═══════════════════════════════════════════════════════════════════════════════

# AWS credentials (if not using IAM roles)
# AWS_ACCESS_KEY_ID=your-access-key
# AWS_SECRET_ACCESS_KEY=your-secret-key
# AWS_REGION=us-east-1

# MinIO or S3-compatible
# S3_ENDPOINT=http://localhost:9000
# S3_ACCESS_KEY=minioadmin
# S3_SECRET_KEY=minioadmin
`
}
