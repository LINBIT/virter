package cmd

import (
	"reflect"
	"testing"
)

func TestParseArgMap(t *testing.T) {
	cases := []struct {
		input       string
		expect      map[string]string
		expectError bool
	}{
		{
			input:  "",
			expect: map[string]string{},
		}, {
			input: "name=test,size=5GiB,format=qcow2,bus=virtio",
			expect: map[string]string{
				"name":   "test",
				"size":   "5GiB",
				"format": "qcow2",
				"bus":    "virtio",
			},
		}, {
			input: "name=test",
			expect: map[string]string{
				"name": "test",
			},
		}, {
			input:  ",,,",
			expect: map[string]string{},
		}, {
			input: ",name=test,",
			expect: map[string]string{
				"name": "test",
			},
		}, {
			input:       "x,y,z",
			expectError: true,
		}, {
			input: "name=",
			expect: map[string]string{
				"name": "",
			},
		}, {
			input:       "=test",
			expectError: true,
		}, {
			input:       "name=test=hello",
			expectError: true,
		}, {
			input: "name=test hello",
			expect: map[string]string{
				"name": "test hello",
			},
		}, {
			input: "name=test,name=hello",
			expect: map[string]string{
				"name": "hello",
			},
		},
	}

	for _, c := range cases {
		actual, err := parseArgMap(c.input)
		if !c.expectError && err != nil {
			t.Errorf("on input '%s':", c.input)
			t.Fatalf("unexpected error: %v", err)
		}
		if c.expectError && err == nil {
			t.Errorf("on input '%s':", c.input)
			t.Fatal("expected error, got nil")
		}

		if !reflect.DeepEqual(actual, c.expect) {
			t.Errorf("on input '%s':a", c.input)
			t.Errorf("unexpected map contents")
			t.Errorf("expected: %+v", c.expect)
			t.Errorf("actual: %+v", actual)
		}
	}
}
