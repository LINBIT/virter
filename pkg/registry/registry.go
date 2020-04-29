package registry

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
)

type imageEntry struct {
	URL string `toml:"url"`
}

type ImageRegistry struct {
	sources []string
	entries map[string]imageEntry
}

func New(files ...string) *ImageRegistry {
	log.Debugf("New image registry from files: %v", files)
	return &ImageRegistry{
		sources: files,
		entries: nil,
	}
}

func (r *ImageRegistry) load() error {
	entries := make(map[string]imageEntry, 0)

	for _, f := range r.sources {
		log.Debugf("Loading image registry file: %s", f)
		var fileEntries map[string]imageEntry
		_, err := toml.DecodeFile(f, &fileEntries)
		if err != nil {
			if os.IsNotExist(err) {
				// ignore nonexistent files
				continue
			}
			return fmt.Errorf("failed to decode image registry file '%v': %w", f, err)
		}

		for k, v := range fileEntries {
			// if the key already exists, overwrite it
			entries[k] = v
		}
	}

	r.entries = entries

	return nil
}

func (r *ImageRegistry) Lookup(imageName string) (string, error) {
	if err := r.load(); err != nil {
		return "", fmt.Errorf("failed to load image registry: %w", err)
	}
	entry, ok := r.entries[imageName]
	if !ok {
		return "", fmt.Errorf("image %v not found in registry", imageName)
	}

	return entry.URL, nil
}

func (r *ImageRegistry) List() (map[string]imageEntry, error) {
	if err := r.load(); err != nil {
		return nil, fmt.Errorf("failed to load image registry: %w", err)
	}
	return r.entries, nil
}
