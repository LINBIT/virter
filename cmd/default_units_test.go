package cmd

import (
	"testing"

	"github.com/rck/unit"
)

func TestDefaultUnits(t *testing.T) {
	cases := []struct {
		input  string
		expect int64
		err    bool
	}{
		{input: "2048", expect: 2048 * unit.M},
		{input: "2048M", expect: 2048 * unit.M},
		{input: "2G", expect: 2 * unit.G},
		{input: "+2048", expect: 2048 * unit.M},
		{input: "2048X", err: true},
		{input: "", err: true},
	}
	for _, c := range cases {
		v, err := memoryUnit.ValueFromString(c.input)
		if c.err {
			if err == nil {
				t.Errorf("%q: expected error", c.input)
			}
			continue
		}
		if err != nil || v.Value != c.expect {
			t.Errorf("%q: got %v, %v, want %d", c.input, v, err, c.expect)
		}
	}
}
