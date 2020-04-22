package virter

import (
	"strings"
	"testing"
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
		{validGlobalOnly, true, ProvisionOption{Values: []string{}}, []string{"foo=bar", "bar=baz"}},
		{validLocalOnly, true, ProvisionOption{Values: []string{}}, []string{"foo=bar", "bar=baz=lala"}},
		{bothDistinct, true, ProvisionOption{Values: []string{}}, []string{"foo=bar", "bar=baz", "rck=was", "here="}},
		{bothOverride, true, ProvisionOption{Values: []string{}}, []string{"foo=rck", "bar=baz", "rck=was"}},
		{"", true, ProvisionOption{
			Values: []string{"steps[0].shell.script=env", "steps[0].shell.env.foo=bar"},
		}, []string{"foo=bar"}},
		{"", true, ProvisionOption{
			Values: []string{"steps[0].shell.script=env", "env.foo=bar", "steps[0].shell.env.foo=rck"},
		}, []string{"foo=rck"}},
		{bothOverride, true, ProvisionOption{
			Values: []string{"steps[0].shell.script=env", "steps[0].shell.env.foo=xyz"},
		}, []string{"foo=xyz", "bar=baz", "rck=was"}},
	}

	for i, tc := range tests {
		r := strings.NewReader(tc.input)
		pc, err := newProvisionConfigReader(r, tc.provOpts)

		if err != nil {
			if tc.valid {
				t.Errorf("Expexted test %d to be valid", i)
			}
			continue // err but also expected to be not valid
		}

		e1, e2 := EnvmapToSlice(pc.Steps[0].Shell.Env), tc.expected
		if !envEqual(e1, e2) {
			t.Errorf("Expexted test %d cfg env (%q) and generated env (%q) to be equal", i, e2, e1)
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
