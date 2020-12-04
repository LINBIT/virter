package sshkeys_test

import (
	"strings"
	"testing"

	"github.com/LINBIT/virter/pkg/sshkeys"
	"github.com/stretchr/testify/assert"
)

func TestKnownHosts_AsFile(t *testing.T) {
	cases := []struct {
		name          string
		hostkeys      map[string][]string
		expectedLines []string
	}{
		{
			name:          "empty",
			hostkeys:      nil,
			expectedLines: nil,
		},
		{
			name: "filled",
			hostkeys: map[string][]string{
				"ssh-rsa key1\n":     {"10.0.0.0", "key1.test"},
				"ssh-ecdsa abcdef\n": {"host"},
			},
			expectedLines: []string{
				"10.0.0.0,key1.test ssh-rsa key1\n",
				"host ssh-ecdsa abcdef\n",
			},
		},
	}

	for i := range cases {
		testcase := &cases[i]
		t.Run(testcase.name, func(t *testing.T) {
			knownHosts := sshkeys.NewKnownHosts()

			for key, entries := range testcase.hostkeys {
				knownHosts.AddHost(key, entries...)
			}

			builder := strings.Builder{}
			err := knownHosts.AsKnownHostsFile(&builder)
			assert.NoError(t, err)

			actual := builder.String()
			var actualLines []string
			for _, line := range strings.SplitAfter(actual, "\n") {
				if line == "" || line == "\n" {
					continue
				}
				actualLines = append(actualLines, line)
			}

			assert.ElementsMatch(t, testcase.expectedLines, actualLines)
		})
	}
}
