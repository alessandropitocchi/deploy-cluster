package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/provider"
	"github.com/alepito/deploy-cluster/pkg/provider/existing"
	"github.com/alepito/deploy-cluster/pkg/provider/k3d"
	"github.com/alepito/deploy-cluster/pkg/provider/kind"
	"github.com/alepito/deploy-cluster/pkg/template"
)

func getProvider(providerType string) (provider.Provider, error) {
	switch providerType {
	case "kind":
		return kind.New(), nil
	case "k3d":
		return k3d.New(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}

func getProviderFromTemplate(cfg *template.Template) (provider.Provider, error) {
	switch cfg.Provider.Type {
	case "kind":
		return kind.New(), nil
	case "k3d":
		return k3d.New(), nil
	case "existing":
		return existing.New(cfg.Provider.Kubeconfig, cfg.Provider.Context), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider.Type)
	}
}
