package netcopy_test

import (
	"github.com/LINBIT/virter/pkg/netcopy"
	"testing"
)

func TestParseHostPath(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name     string
		value    string
		expected netcopy.HostPath
	}{
		{
			name:     "local-path-absolute",
			value:    "/some/extra/long/path",
			expected: netcopy.HostPath{Path: "/some/extra/long/path"},
		},
		{
			name:     "local-path-relative",
			value:    "extra/long/relative/path",
			expected: netcopy.HostPath{Path: "extra/long/relative/path"},
		},
		{
			name:     "local-path-with-colon",
			value:    "some/strange:path/with:colon",
			expected: netcopy.HostPath{Path: "some/strange:path/with:colon"},
		},
		{
			name:     "remote-path-absolute",
			value:    "remote:/some/extra/long/path",
			expected: netcopy.HostPath{Path: "/some/extra/long/path", Host: "remote"},
		},
		{
			name:     "remote-path-absolute-with-colon",
			value:    "remote:/some/strange:path",
			expected: netcopy.HostPath{Path: "/some/strange:path", Host: "remote"},
		},
		{
			name:     "remote-path-relative",
			value:    "remote:relative",
			expected: netcopy.HostPath{Path: "relative", Host: "remote"},
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			actual := netcopy.ParseHostPath(test.value)

			if actual.Host != test.expected.Host {
				t.Errorf(".Host does not match: expected: %s, actual: %s", test.expected.Host, actual.Host)
			}

			if actual.Path != test.expected.Path {
				t.Errorf(".Path does not match: expected: %s, actual: %s", test.expected.Path, actual.Path)
			}
		})
	}
}
