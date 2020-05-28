package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/LINBIT/virter/pkg/registry"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

const upstreamRegistryURL = "https://linbit.github.io/virter/images.toml"

func defaultRegistryPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal(err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "virter")
}

func userRegistryFile() string {
	return filepath.Join(configPath(), "images.toml")
}

func fetchShippedRegistry(path string) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create shipped registry directory ('%v'): %w",
			dir, err)
	}

	log.Debugf("Fetching shipped registry from '%s'", upstreamRegistryURL)

	resp, err := http.Get(upstreamRegistryURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %s", resp.Status)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func loadRegistry() *registry.ImageRegistry {
	registryPath := filepath.Join(defaultRegistryPath(), "images.toml")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		log.Infof("Builtin image registry does not exist, writing to %v", registryPath)
		err := fetchShippedRegistry(registryPath)
		if err != nil {
			log.Warnf("Failed to fetch builtin image registry file: %v", err)
			log.Warnf("Proceeding with only user-defined images")
		}
	}

	return registry.New(registryPath, userRegistryFile())
}
