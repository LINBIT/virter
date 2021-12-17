package virter

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kr/pretty"

	"github.com/LINBIT/virter/pkg/pullpolicy"
)

func TestNewProvisionConfig(t *testing.T) {
	validGlobalOnly := `
[env]
foo="bar"
bar="baz"

[[steps]]
[steps.shell]
script = "echo rck"
`

	validLocalOnly := `
[[steps]]
[steps.shell]
script = "echo rck"
[steps.shell.env]
foo = "bar"
bar="baz=lala"
`

	bothDistinct := `
[env]
foo="bar"
bar="baz"

[[steps]]
[steps.shell]
script = "echo rck"
[steps.shell.env]
rck="was"
here=""
`

	bothOverride := `
[env]
foo="bar"
bar="baz"

[[steps]]
[steps.shell]
script = "echo rck"
[steps.shell.env]
rck="was"
foo="rck"
`

	// IMPORTANT: this asumes 1 shell step!
	tests := []struct {
		input    string
		valid    bool
		provOpts ProvisionOption
		expected []string
	}{
		{validGlobalOnly, true, ProvisionOption{Overrides: []string{}}, []string{"foo=bar", "bar=baz"}},
		{validLocalOnly, true, ProvisionOption{Overrides: []string{}}, []string{"foo=bar", "bar=baz=lala"}},
		{bothDistinct, true, ProvisionOption{Overrides: []string{}}, []string{"foo=bar", "bar=baz", "rck=was", "here="}},
		{bothOverride, true, ProvisionOption{Overrides: []string{}}, []string{"foo=rck", "bar=baz", "rck=was"}},
		{"", true, ProvisionOption{
			Overrides: []string{"steps[0].shell.script=env", "steps[0].shell.env.foo=bar"},
		}, []string{"foo=bar"}},
		{"", true, ProvisionOption{
			Overrides: []string{"steps[0].shell.script=env", "env.foo=bar", "steps[0].shell.env.foo=rck"},
		}, []string{"foo=rck"}},
		{bothOverride, true, ProvisionOption{
			Overrides: []string{"steps[0].shell.script=env", "steps[0].shell.env.foo=xyz"},
		}, []string{"foo=xyz", "bar=baz", "rck=was"}},
	}

	for i, tc := range tests {
		r := strings.NewReader(tc.input)
		pc, err := newProvisionConfigReader(r, tc.provOpts)

		if !tc.valid && err == nil {
			t.Errorf("Expexted test %d to be invalid", i)
		}
		if tc.valid {
			if err != nil {
				t.Errorf("Expected test %d to be valid, got error: %+v", i, err)
			} else {
				e1, e2 := EnvmapToSlice(pc.Steps[0].Shell.Env), tc.expected
				if !envEqual(e1, e2) {
					t.Errorf("Expexted test %d cfg env (%q) and generated env (%q) to be equal", i, e2, e1)
				}
			}
		}
	}
}

func envEqual(env, expected []string) bool {
	if len(env) != len(expected) {
		return false
	}

	for _, en := range env {
		found := false
		for _, exp := range expected {
			if en == exp {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func TestNewProvisionConfigTemplate(t *testing.T) {
	noTemplate := `
[[steps]]
[steps.docker]
image = "some-image"
command = ["exit", "0"]
[steps.docker.env]
foo = "bar"

[[steps]]
[steps.shell]
script = "echo jrc"
[steps.shell.env]
foo2 = "bar2"

[[steps]]
[steps.rsync]
source = "some-source"
dest = "some-dest"
`

	allTemplate := `
[values]
ShellEnv = "default-value"

[[steps]]
[steps.docker]
image = "{{.DockerImage}}"
pull = "Always"
command = ["echo", "{{.DockerCommandArg}}"]
[steps.docker.env]
foo = "hello {{.DockerEnv}}"

[[steps]]
[steps.shell]
script = "echo jrc"
[steps.shell.env]
foo2 = "{{.ShellEnv}}"

[[steps]]
[steps.rsync]
source = "{{.RsyncSource}}"
dest = "some-dest"
`

	globalEnvTemplate := `
[env]
foo = "{{.Env}}"
blah = "{{.MoreEnv}}"

[[steps]]
[steps.shell]
script = "echo jrc"
[steps.shell.env]
foo = "bar"
`

	brokenTemplate := `
[[steps]]
[steps.shell]
script = "echo jrc"
[steps.shell.env]
foo = "{{.ShellEnv"
`

	tests := []struct {
		description string
		input       string
		valid       bool
		provOpts    ProvisionOption
		expected    []ProvisionStep
	}{
		{
			"no-template", noTemplate, true, ProvisionOption{}, []ProvisionStep{
				ProvisionStep{
					Docker: &ProvisionDockerStep{
						Image:   "some-image",
						Command: []string{"exit", "0"},
						Env:     map[string]string{"foo": "bar"},
						Pull:    "",
					},
				},
				ProvisionStep{
					Shell: &ProvisionShellStep{
						Script: "echo jrc",
						Env:    map[string]string{"foo2": "bar2"},
					},
				},
				ProvisionStep{
					Rsync: &ProvisionRsyncStep{
						Source: "some-source",
						Dest:   "some-dest",
					},
				},
			},
		},
		{
			"all-template", allTemplate, true,
			ProvisionOption{
				Overrides: []string{
					"values.DockerImage=template-image",
					"values.DockerEnv=template-value",
					"values.DockerCommandArg=template-arg",
					"values.RsyncSource=template-source",
				},
			},
			[]ProvisionStep{
				ProvisionStep{
					Docker: &ProvisionDockerStep{
						Image:   "template-image",
						Command: []string{"echo", "template-arg"},
						Env:     map[string]string{"foo": "hello template-value"},
						Pull:    pullpolicy.Always,
					},
				},
				ProvisionStep{
					Shell: &ProvisionShellStep{
						Script: "echo jrc",
						Env:    map[string]string{"foo2": "default-value"},
					},
				},
				ProvisionStep{
					Rsync: &ProvisionRsyncStep{
						Source: "template-source",
						Dest:   "some-dest",
					},
				},
			},
		},
		{
			"global-env-template", globalEnvTemplate, true,
			ProvisionOption{
				Overrides: []string{
					"values.Env=template-value",
					"values.MoreEnv=jrc",
				},
			},
			[]ProvisionStep{
				ProvisionStep{
					Shell: &ProvisionShellStep{
						Script: "echo jrc",
						Env:    map[string]string{"foo": "bar", "blah": "jrc"},
					},
				},
			},
		},
		{
			"broken-template", brokenTemplate, false, ProvisionOption{}, nil,
		},
	}

	for _, tc := range tests {
		r := strings.NewReader(tc.input)
		pc, err := newProvisionConfigReader(r, tc.provOpts)

		if !tc.valid && err == nil {
			t.Errorf("did not get expected error for test %s", tc.description)
		}
		if tc.valid {
			if err != nil {
				t.Errorf("unexpected error for test %s: %+v", tc.description, err)
			} else {
				if !reflect.DeepEqual(pc.Steps, tc.expected) {
					t.Errorf("unexpected result for test %s:", tc.description)
					pretty.Ldiff(t, tc.expected, pc.Steps)
				}
			}
		}
	}
}
