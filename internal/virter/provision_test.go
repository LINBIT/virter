package virter

import (
	"strings"
	"testing"
)

func TestNewProvisionConfig(t *testing.T) {
	validGlobalOnly := `
env = ['foo=bar', 'bar=baz']
[[steps]]
[steps.shell]
script = "echo rck"
`

	validLocalOnly := `
[[steps]]
[steps.shell]
env = ['foo=bar', 'bar=baz=lala']
script = "echo rck"
`

	bothDistinct := `
env = ['foo=bar', 'bar=baz']
[[steps]]
[steps.shell]
env = ['rck=was', 'here=']
script = "echo rck"
`

	bothOverride := `
env = ['foo=bar', 'bar=baz']
[[steps]]
[steps.shell]
env = ['rck=was', 'foo=rck']
script = "echo rck"
`

	// IMPORTANT: this asumes 1 shell step!
	tests := []struct {
		input    string
		valid    bool
		expected []string
	}{
		{validGlobalOnly, true, []string{"foo=bar", "bar=baz"}},
		{validLocalOnly, true, []string{"foo=bar", "bar=baz=lala"}},
		{bothDistinct, true, []string{"foo=bar", "bar=baz", "rck=was", "here="}},
		{bothOverride, true, []string{"foo=rck", "bar=baz", "rck=was"}},
	}

	for i, tc := range tests {
		r := strings.NewReader(tc.input)
		pc, err := NewProvisionConfig(r)
		if err != nil && tc.valid {
			t.Errorf("Expexted test %d to be valid", i)
		}
		e1, e2 := pc.Steps[0].Shell.Env, tc.expected
		if !envEqual(e1, e2) {
			t.Errorf("Expexted test %d cfg env (%q) and generated env (%q) to be equal", i, e1, e2)
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
