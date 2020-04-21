package virter

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/helm/helm/pkg/strvals"
	"github.com/mitchellh/mapstructure"
)

// ProvisionDockerStep is a single provisioniong step executed in a docker container
type ProvisionDockerStep struct {
	Image string   `toml:"image"`
	Env   []string `toml:"env"`
}

// ProvisionShellStep is a single provisioniong step executed in a shell (via ssh)
type ProvisionShellStep struct {
	Script string   `toml:"script"`
	Env    []string `toml:"env"`
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
	Env   []string        `toml:"env"`
	Steps []ProvisionStep `toml:"steps"`
}

// getEnvString returns an env string "foo=bar" as "foo", "bar" and checks for a limited number of errors
func getEnvString(kv string) (string, string, error) {
	var k, v string

	if !strings.Contains(kv, "=") {
		return k, v, fmt.Errorf("There is no '=' in this env string")
	}
	kvs := strings.SplitN(kv, "=", 2)
	if len(kvs) == 1 { // "FOO="
		kvs = append(kvs, "")
	}
	// now we need to have 2
	if len(kvs) != 2 {
		return k, v, fmt.Errorf("Got malformed shell variable: '%s'", kv)
	}
	k, v = kvs[0], kvs[1]

	return k, v, nil
}

// getEnvMap takes a slice of env variables and turns them into am k, v map
func getEnvMap(env []string) (map[string]string, error) {
	e := make(map[string]string)

	for _, kv := range env {
		k, v, err := getEnvString(kv)
		if err != nil {
			return e, err
		}
		e[k] = v
	}

	return e, nil
}

// mergeEnv takes two pointers to env Maps and merges them, lower keys overriding upper ones
func mergeEnv(upper, lower *map[string]string) []string {
	envMap := make(map[string]string)

	for k, v := range *upper {
		envMap[k] = v
	}
	for k, v := range *lower {
		envMap[k] = v
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
	var r io.ReadCloser
	if provOpt.FilePath != "" {
		r, err := os.Open(provOpt.FilePath)
		if err != nil {
			return ProvisionConfig{}, err
		}
		defer r.Close()
	}

	return newProvisionConfigReader(r, provOpt)
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

	// do not early return if len(pc.Env) == 0;
	// we still want to run the checks for the "lower"/in Steps.*.Env variables

	globalEnv, err := getEnvMap(pc.Env)
	if err != nil {
		return pc, err
	}

	for _, s := range pc.Steps {
		if s.Docker != nil {
			localEnv, err := getEnvMap(s.Docker.Env)
			if err != nil {
				return pc, err
			}
			s.Docker.Env = mergeEnv(&globalEnv, &localEnv)
		} else if s.Shell != nil {
			localEnv, err := getEnvMap(s.Shell.Env)
			if err != nil {
				return pc, err
			}
			s.Shell.Env = mergeEnv(&globalEnv, &localEnv)
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
