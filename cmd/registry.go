package cmd

import (
	"github.com/LINBIT/virter/pkg/registry"
	"github.com/spf13/viper"
)

func userRegistryFile() string {
	return viper.GetString("image.registry")
}

func loadRegistry() *registry.ImageRegistry {
	return registry.New(userRegistryFile())
}
