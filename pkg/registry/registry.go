package registry

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
)

const CurrentRegistryFileVersion = 1

var (
	ErrNotFound = errors.New("not found")
)

type imageEntry struct {
	URL string `toml:"url"`
}

type registryFile struct {
	Version int                    `toml:"version"`
	Images  map[string]imageEntry  `toml:"images"`
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
		var rf registryFile
		md, err := toml.DecodeFile(f, &rf)
		if err != nil {
			if os.IsNotExist(err) {
				// ignore nonexistent files
				continue
			}
			return fmt.Errorf("failed to decode image registry file '%v': %w", f, err)
		}

		for _, k := range md.Undecoded() {
			log.WithField("key", k).Warn("Unknown key in image registry file")
		}

		if rf.Version != CurrentRegistryFileVersion {
			return fmt.Errorf("unsupported registry file version %d in '%v' (want %d)", rf.Version, f, CurrentRegistryFileVersion)
		}

		for k, v := range rf.Images {
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
		return "", fmt.Errorf("could not look up image %v in registry: %w", imageName, ErrNotFound)
	}

	return entry.URL, nil
}

func (r *ImageRegistry) List() (map[string]imageEntry, error) {
	if err := r.load(); err != nil {
		return nil, fmt.Errorf("failed to load image registry: %w", err)
	}
	return r.entries, nil
}
