package virter

type DockerStep struct {
	Image string   `toml:"image"`
	Env   []string `toml:"env"`
}

type ShellStep struct {
	Script string   `toml:"script"`
	Env    []string `toml:"env"`
}

type Step struct {
	Docker *DockerStep `toml:"docker,omitempty"`
	Shell  *ShellStep  `toml:"shell,omitempty"`
}

type ProvisionConfig struct {
	Memory string `toml:"memory"`
	Steps  []Step `toml:"steps"`
}
