package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/provider"
	"github.com/alepito/deploy-cluster/pkg/provider/k3d"
	"github.com/alepito/deploy-cluster/pkg/provider/kind"
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
