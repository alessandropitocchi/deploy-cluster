// Package plugin provides orchestration for plugin installation.
package plugin

import (
	"fmt"
	"sort"
	"sync"

	"github.com/alepito/deploy-cluster/pkg/logger"
)

// Manager orchestrates plugin operations.
type Manager struct {
	registry    *Registry
	log         *logger.Logger
	failFast    bool
	parallel    bool
	maxParallel int
}

// ManagerOption configures the Manager.
type ManagerOption func(*Manager)

// WithFailFast stops at the first plugin failure.
func WithFailFast(failFast bool) ManagerOption {
	return func(m *Manager) {
		m.failFast = failFast
	}
}

// WithParallel enables parallel plugin installation.
func WithParallel(parallel bool) ManagerOption {
	return func(m *Manager) {
		m.parallel = parallel
	}
}

// WithMaxParallel sets the maximum number of parallel plugins.
func WithMaxParallel(max int) ManagerOption {
	return func(m *Manager) {
		m.maxParallel = max
	}
}

// NewManager creates a new plugin manager.
func NewManager(registry *Registry, log *logger.Logger, opts ...ManagerOption) *Manager {
	m := &Manager{
		registry:    registry,
		log:         log,
		failFast:    false,
		parallel:    false,
		maxParallel: 3,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// InstallConfig holds configuration for plugin installation.
type InstallConfig struct {
	Kubecontext  string
	ProviderType string
}

// Install installs a single plugin.
func (m *Manager) Install(pluginName string, cfg interface{}, opts InstallConfig) InstallResult {
	plugin, ok := m.registry.Get(pluginName)
	if !ok {
		return InstallResult{
			Name: pluginName,
			Err:  fmt.Errorf("plugin %s not found in registry", pluginName),
		}
	}

	// Check if already installed
	installed, err := plugin.IsInstalled(opts.Kubecontext)
	if err != nil {
		return InstallResult{
			Name: pluginName,
			Err:  fmt.Errorf("failed to check installation status: %w", err),
		}
	}

	if installed {
		m.log.Info("[%s] Already installed, skipping...\n", pluginName)
		return InstallResult{
			Name:    pluginName,
			Skipped: true,
		}
	}

	// Install the plugin
	if err := plugin.Install(cfg, opts.Kubecontext, opts.ProviderType); err != nil {
		return InstallResult{
			Name: pluginName,
			Err:  err,
		}
	}

	return InstallResult{
		Name:    pluginName,
		Skipped: false,
	}
}

// InstallAll installs multiple plugins in the correct order.
func (m *Manager) InstallAll(plugins []struct {
	Name    string
	Config  interface{}
	Enabled bool
}, opts InstallConfig) []InstallResult {
	// Filter enabled plugins and sort by install order
	enabledPlugins := filterAndSortPlugins(plugins)

	if m.parallel {
		return m.installParallel(enabledPlugins, opts)
	}
	return m.installSequential(enabledPlugins, opts)
}

// installSequential installs plugins one by one.
func (m *Manager) installSequential(plugins []pluginConfig, opts InstallConfig) []InstallResult {
	var results []InstallResult

	for _, p := range plugins {
		m.log.Info("\n")
		result := m.Install(p.Name, p.Config, opts)
		results = append(results, result)

		if result.Err != nil && m.failFast {
			return results
		}
	}

	return results
}

// installParallel installs independent plugins in parallel.
// Plugins with dependencies are still installed sequentially.
func (m *Manager) installParallel(plugins []pluginConfig, opts InstallConfig) []InstallResult {
	// For now, we install in groups based on dependencies
	// Group 1: storage, ingress (foundational)
	// Group 2: cert-manager (depends on nothing, but others may depend on it)
	// Group 3: monitoring, dashboard, custom-apps (independent)
	// Group 4: argocd (depends on everything)

	groups := [][]string{
		{"storage", "ingress"},
		{"cert-manager"},
		{"monitoring", "dashboard", "custom-apps"},
		{"argocd"},
	}

	var results []InstallResult
	resultsMu := sync.Mutex{}

	for _, group := range groups {
		var groupPlugins []pluginConfig
		for _, name := range group {
			for _, p := range plugins {
				if p.Name == name {
					groupPlugins = append(groupPlugins, p)
					break
				}
			}
		}

		if len(groupPlugins) == 0 {
			continue
		}

		if len(groupPlugins) == 1 || !m.parallel {
			// Sequential within group
			for _, p := range groupPlugins {
				m.log.Info("\n")
				result := m.Install(p.Name, p.Config, opts)
				resultsMu.Lock()
				results = append(results, result)
				resultsMu.Unlock()

				if result.Err != nil && m.failFast {
					return results
				}
			}
		} else {
			// Parallel within group
			results = m.installGroupParallel(groupPlugins, opts, results)
		}
	}

	return results
}

func (m *Manager) installGroupParallel(plugins []pluginConfig, opts InstallConfig, currentResults []InstallResult) []InstallResult {
	var wg sync.WaitGroup
	resultsChan := make(chan InstallResult, len(plugins))

	// Use semaphore to limit parallelism
	sem := make(chan struct{}, m.maxParallel)

	for _, p := range plugins {
		wg.Add(1)
		go func(plugin pluginConfig) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			m.log.Info("\n")
			result := m.Install(plugin.Name, plugin.Config, opts)
			resultsChan <- result
		}(p)
	}

	wg.Wait()
	close(resultsChan)

	var results []InstallResult
	for r := range resultsChan {
		results = append(results, r)
		if r.Err != nil && m.failFast {
			// Drain remaining results
			for r := range resultsChan {
				results = append(results, r)
			}
			break
		}
	}

	return append(currentResults, results...)
}

// Upgrade upgrades a single plugin.
func (m *Manager) Upgrade(pluginName string, cfg interface{}, opts InstallConfig) InstallResult {
	plugin, ok := m.registry.Get(pluginName)
	if !ok {
		return InstallResult{
			Name: pluginName,
			Err:  fmt.Errorf("plugin %s not found in registry", pluginName),
		}
	}

	installed, err := plugin.IsInstalled(opts.Kubecontext)
	if err != nil {
		return InstallResult{
			Name: pluginName,
			Err:  fmt.Errorf("failed to check installation status: %w", err),
		}
	}

	if !installed {
		// Not installed, do fresh install
		m.log.Info("[%s] Not installed, performing installation...\n", pluginName)
		return m.Install(pluginName, cfg, opts)
	}

	// Upgrade
	if err := plugin.Upgrade(cfg, opts.Kubecontext, opts.ProviderType); err != nil {
		return InstallResult{
			Name: pluginName,
			Err:  err,
		}
	}

	return InstallResult{
		Name: pluginName,
	}
}

// UpgradeAll upgrades multiple plugins.
func (m *Manager) UpgradeAll(plugins []struct {
	Name    string
	Config  interface{}
	Enabled bool
}, opts InstallConfig) []InstallResult {
	enabledPlugins := filterAndSortPlugins(plugins)

	var results []InstallResult
	for _, p := range enabledPlugins {
		m.log.Info("\n")
		result := m.Upgrade(p.Name, p.Config, opts)
		results = append(results, result)

		if result.Err != nil && m.failFast {
			return results
		}
	}

	return results
}

// DryRun performs a dry run for all enabled plugins.
func (m *Manager) DryRun(plugins []struct {
	Name    string
	Config  interface{}
	Enabled bool
}, opts InstallConfig) []InstallResult {
	enabledPlugins := filterAndSortPlugins(plugins)

	var results []InstallResult
	for _, p := range enabledPlugins {
		plugin, ok := m.registry.Get(p.Name)
		if !ok {
			results = append(results, InstallResult{
				Name: p.Name,
				Err:  fmt.Errorf("plugin not found"),
			})
			continue
		}

		if err := plugin.DryRun(p.Config, opts.Kubecontext, opts.ProviderType); err != nil {
			results = append(results, InstallResult{
				Name: p.Name,
				Err:  err,
			})
		} else {
			results = append(results, InstallResult{
				Name: p.Name,
			})
		}
	}

	return results
}

// pluginConfig is an internal struct for plugin configuration.
type pluginConfig struct {
	Name   string
	Config interface{}
}

// filterAndSortPlugins filters enabled plugins and sorts them by install order.
func filterAndSortPlugins(plugins []struct {
	Name    string
	Config  interface{}
	Enabled bool
}) []pluginConfig {
	var enabled []pluginConfig
	for _, p := range plugins {
		if p.Enabled {
			enabled = append(enabled, pluginConfig{
				Name:   p.Name,
				Config: p.Config,
			})
		}
	}

	// Sort by install order
	sort.Slice(enabled, func(i, j int) bool {
		return GetInstallOrder(enabled[i].Name) < GetInstallOrder(enabled[j].Name)
	})

	return enabled
}

// HasErrors checks if any result has an error.
func HasErrors(results []InstallResult) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// CountSuccessful returns the number of successful installations.
func CountSuccessful(results []InstallResult) int {
	count := 0
	for _, r := range results {
		if r.Err == nil && !r.Skipped {
			count++
		}
	}
	return count
}

// CountSkipped returns the number of skipped installations.
func CountSkipped(results []InstallResult) int {
	count := 0
	for _, r := range results {
		if r.Skipped {
			count++
		}
	}
	return count
}

// CountFailed returns the number of failed installations.
func CountFailed(results []InstallResult) int {
	count := 0
	for _, r := range results {
		if r.Err != nil {
			count++
		}
	}
	return count
}
