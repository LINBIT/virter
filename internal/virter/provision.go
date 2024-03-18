package virter

import (
	"bytes"
	"fmt"
	"io"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/helm/helm/pkg/strvals"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/virter/pkg/pullpolicy"
)

const CurrentProvisionFileVersion = 1

// ProvisionContainerStep is a single provisioning step executed in a container
type ProvisionContainerStep struct {
	Image   string                      `toml:"image"`
	Pull    pullpolicy.PullPolicy       `toml:"pull"`
	Env     map[string]string           `toml:"env"`
	Command []string                    `toml:"command"`
	Copy    *ProvisionContainerCopyStep `toml:"copy"`
}

type ProvisionContainerCopyStep struct {
	Source string `toml:"source"`
	Dest   string `toml:"dest"`
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

// ProvisionStep is a single provisioning step
type ProvisionStep struct {
	Container *ProvisionContainerStep `toml:"container,omitempty"`
	Shell     *ProvisionShellStep     `toml:"shell,omitempty"`
	Rsync     *ProvisionRsyncStep     `toml:"rsync,omitempty"`
}

// ProvisionConfig holds the configuration of the whole provisioning
type ProvisionConfig struct {
	Version int               `toml:"version"`
	Values  map[string]string `toml:"values"`
	Env     map[string]string `toml:"env"`
	Steps   []ProvisionStep   `toml:"steps"`
}

// NeedsContainers checks if there is a provision step that requires a container provider (like Docker or Podman)
func (p *ProvisionConfig) NeedsContainers() bool {
	for _, s := range p.Steps {
		if s.Container != nil {
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
	Overrides          []string
	DefaultPullPolicy  pullpolicy.PullPolicy
	OverridePullPolicy pullpolicy.PullPolicy
}

// NewProvisionConfig returns a ProvisionConfig from a ProvisionOption
func NewProvisionConfig(reader io.ReadCloser, provOpt ProvisionOption) (ProvisionConfig, error) {
	return newProvisionConfigReader(reader, provOpt)
}

// newProvisionConfigReader returns a ProvisionConfig and does some necesary checks and for example merges the global env to the individual steps.
func newProvisionConfigReader(provReader io.ReadCloser, provOpt ProvisionOption) (ProvisionConfig, error) {
	var pc ProvisionConfig

	if provReader != nil {
		defer provReader.Close()

		decoder := toml.NewDecoder(provReader)
		md, err := decoder.Decode(&pc)
		if err != nil {
			return pc, err
		}

		for _, k := range md.Undecoded() {
			log.WithField("key", k).Warn("Unknown key in provisioning")
		}
	}

	m, err := genValueMap(provOpt)
	if err != nil {
		return pc, err
	}
	if err := mapstructure.Decode(m, &pc); err != nil {
		return pc, err
	}

	if pc.Version != CurrentProvisionFileVersion {
		return pc, fmt.Errorf("unsupported provision file version %d (want %d)", pc.Version, CurrentProvisionFileVersion)
	}

	for i, s := range pc.Steps {
		if s.Container != nil {
			s.Container.Env = mergeEnv(&pc.Env, &s.Container.Env)

			if s.Container.Image, err = executeTemplate(s.Container.Image, pc.Values); err != nil {
				return pc, fmt.Errorf("failed to execute template for container.image for step %d: %w", i, err)
			}

			if err := executeTemplateMap(s.Container.Env, pc.Values); err != nil {
				return pc, fmt.Errorf("failed to execute template for container.env for step %d: %w", i, err)
			}

			if err := executeTemplateArray(s.Container.Command, pc.Values); err != nil {
				return pc, fmt.Errorf("failed to execute tempalte for container.command for step %d: %w", i, err)
			}

			if copyStep := s.Container.Copy; copyStep != nil {
				if copyStep.Dest, err = executeTemplate(copyStep.Dest, pc.Values); err != nil {
					return pc, fmt.Errorf("failed to execute template for container.copy.dest for step %d: %w", i, err)
				}
			}

			if s.Container.Pull == "" {
				s.Container.Pull = provOpt.DefaultPullPolicy
			}

			if provOpt.OverridePullPolicy != "" {
				s.Container.Pull = provOpt.OverridePullPolicy
			}
		} else if s.Shell != nil {
			s.Shell.Env = mergeEnv(&pc.Env, &s.Shell.Env)

			if err := executeTemplateMap(s.Shell.Env, pc.Values); err != nil {
				return pc, fmt.Errorf("failed to execute template for shell.env for step %d: %w", i, err)
			}
		} else if s.Rsync != nil {
			if s.Rsync.Source, err = executeTemplate(s.Rsync.Source, pc.Values); err != nil {
				return pc, fmt.Errorf("failed to execute template for rsync.source for step %d: %w", i, err)
			}
		}
	}

	return pc, nil
}

func genValueMap(provOpt ProvisionOption) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	for _, value := range provOpt.Overrides {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return base, err
		}
	}

	return base, nil
}

func executeTemplateMap(templates map[string]string, templateData map[string]string) error {
	for k, v := range templates {
		result, err := executeTemplate(v, templateData)
		if err != nil {
			return err
		}
		templates[k] = result
	}
	return nil
}

func executeTemplateArray(templates []string, templateData map[string]string) error {
	for i, t := range templates {
		result, err := executeTemplate(t, templateData)
		if err != nil {
			return err
		}
		templates[i] = result
	}
	return nil
}

func executeTemplate(templateText string, templateData map[string]string) (string, error) {
	tmpl, err := template.New("").Option("missingkey=error").Parse(templateText)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, templateData)
	if err != nil {
		return "", err
	}

	return b.String(), nil
}
