package virter

import (
	"io"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/helm/helm/pkg/strvals"
	"github.com/mitchellh/mapstructure"
)

// ProvisionDockerStep is a single provisioniong step executed in a docker container
type ProvisionDockerStep struct {
	Image string            `toml:"image"`
	Env   map[string]string `toml:"env"`
}

// ProvisionShellStep is a single provisioniong step executed in a shell (via ssh)
type ProvisionShellStep struct {
	Script string            `toml:"script"`
	Env    map[string]string `toml:"env"`
}

// ProvisionRsyncStep is used to copy files to the target via the rsync utility
type ProvisionRsyncStep struct {
	Source string `toml:"source"`
	Dest   string `toml:"dest"`
}

// ProvisionStep is a single provisioniong step
type ProvisionStep struct {
	Docker *ProvisionDockerStep `toml:"docker,omitempty"`
	Shell  *ProvisionShellStep  `toml:"shell,omitempty"`
	Rsync  *ProvisionRsyncStep  `toml:"rsync,omitempty"`
}

// ProvisionConfig holds the configuration of the whole provisioning
type ProvisionConfig struct {
	Env   map[string]string `toml:"env"`
	Steps []ProvisionStep   `toml:"steps"`
}

// NeedsDocker checks if there is a provision step that requires a docker client
func (p *ProvisionConfig) NeedsDocker() bool {
	for _, s := range p.Steps {
		if s.Docker != nil {
			return true
		}
	}
	return false
}

// mergeEnv takes two pointers to env Maps and merges them, lower keys overriding upper ones
func mergeEnv(upper, lower *map[string]string) map[string]string {
	envMap := make(map[string]string)

	for k, v := range *upper {
		envMap[k] = v
	}
	for k, v := range *lower {
		envMap[k] = v
	}
	return envMap
}

func EnvmapToSlice(envMap map[string]string) []string {
	if envMap == nil {
		return []string{}
	}

	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}
	return env
}

// ProvisionOption sumarizes all the options used for generating the final ProvisionConfig
type ProvisionOption struct {
	FilePath string
	Values   []string
}

// NewProvisionConfig returns a ProvisionConfig from a ProvisionOption
func NewProvisionConfig(provOpt ProvisionOption) (ProvisionConfig, error) {
	// file has highest precedence
	var provReader io.ReadCloser
	if provOpt.FilePath != "" {
		r, err := os.Open(provOpt.FilePath)
		if err != nil {
			return ProvisionConfig{}, err
		}
		defer r.Close()
		provReader = r
	}

	return newProvisionConfigReader(provReader, provOpt)
}

// newProvisionConfigReader returns a ProvisionConfig and does some necesary checks and for example merges the global env to the individual steps.
func newProvisionConfigReader(provReader io.Reader, provOpt ProvisionOption) (ProvisionConfig, error) {
	var pc ProvisionConfig

	if provReader != nil {
		_, err := toml.DecodeReader(provReader, &pc)
		if err != nil {
			return pc, err
		}
	}

	m, err := genValueMap(provOpt)
	if err != nil {
		return pc, err
	}
	if err := mapstructure.Decode(m, &pc); err != nil {
		return pc, err
	}

	for _, s := range pc.Steps {
		if s.Docker != nil {
			s.Docker.Env = mergeEnv(&pc.Env, &s.Docker.Env)
		} else if s.Shell != nil {
			s.Shell.Env = mergeEnv(&pc.Env, &s.Shell.Env)
		}
	}

	return pc, nil
}

func genValueMap(provOpt ProvisionOption) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	for _, value := range provOpt.Values {
		if err := strvals.ParseInto(value, base); err != nil {
			return base, err
		}
	}

	return base, nil
}
