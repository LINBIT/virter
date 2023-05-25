package cmd_test

import (
	"github.com/LINBIT/virter/cmd"
	"github.com/rck/unit"
	"reflect"
	"testing"
)

func TestFromFlag(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		expect      cmd.DiskArg
		expectError bool
	}{
		{
			name:        "empty arg is error",
			input:       "",
			expectError: true,
		}, {
			name:  "full spec parses",
			input: "name=test,size=5GiB,format=qcow2,bus=virtio,pool=mypool",
			expect: cmd.DiskArg{
				Name:   "test",
				Size:   cmd.Size{KiB: uint64(5 * unit.G / unit.K)},
				Format: "qcow2",
				Bus:    "virtio",
				Pool:   "mypool",
			},
		}, {
			name:  "only required args parses",
			input: "name=test,size=10G",
			expect: cmd.DiskArg{
				Name:   "test",
				Size:   cmd.Size{KiB: uint64(10 * unit.G / unit.K)},
				Format: "qcow2",
				Bus:    "virtio",
			},
		}, {
			name:        "empty but different is error",
			input:       ",,,",
			expectError: true,
		}, {
			name:  "only required with empty kv-pairs parses",
			input: ",name=test,,,size=1G",
			expect: cmd.DiskArg{
				Name:   "test",
				Size:   cmd.Size{KiB: uint64(1 * unit.G / unit.K)},
				Format: "qcow2",
				Bus:    "virtio",
			},
		}, {
			name:        "nonsence input is error",
			input:       "x,y,z",
			expectError: true,
		}, {
			name:        "missing size is error",
			input:       "name=test",
			expectError: true,
		}, {
			name:        "missing key is error",
			input:       "=test,name=bla,size=1G",
			expectError: true,
		}, {
			name:        "multi-value is error",
			input:       "name=test=hello",
			expectError: true,
		}, {
			name:  "whitepace is okay",
			input: "name=test hello,size=1G",
			expect: cmd.DiskArg{
				Name:   "test hello",
				Size:   cmd.Size{KiB: uint64(1 * unit.G / unit.K)},
				Format: "qcow2",
				Bus:    "virtio",
			},
		}, {
			name:  "repeated keys are okay",
			input: "name=test,name=hello,size=1G",
			expect: cmd.DiskArg{
				Name:   "hello",
				Size:   cmd.Size{KiB: uint64(1 * unit.G / unit.K)},
				Format: "qcow2",
				Bus:    "virtio",
			},
		},
	}

	t.Parallel()
	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			actual := cmd.DiskArg{}
			err := actual.Set(c.input)
			if err != nil {
				if !c.expectError {
					t.Errorf("on input '%s':", c.input)
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if c.expectError {
				t.Errorf("on input '%s':", c.input)
				t.Fatal("expected error, got nil")
			}

			if !reflect.DeepEqual(actual, c.expect) {
				t.Errorf("on input '%s'", c.input)
				t.Errorf("unexpected arg contents")
				t.Errorf("expected: %+v", c.expect)
				t.Errorf("actual: %+v", actual)
			}
		})
	}
}
