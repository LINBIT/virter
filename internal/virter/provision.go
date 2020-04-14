package virter

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

// ProvisionStep is a single provisioniong step
type ProvisionStep struct {
	Docker *ProvisionDockerStep `toml:"docker,omitempty"`
	Shell  *ProvisionShellStep  `toml:"shell,omitempty"`
}

type ProvisionConfig struct {
	Memory string          `toml:"memory"`
	Steps  []ProvisionStep `toml:"steps"`
}
