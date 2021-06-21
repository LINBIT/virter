package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/cmd"
)

func TestLocalImageName(t *testing.T) {
	testcases := []struct {
		image     string
		localname string
	}{
		{
			image:     "centos-8",
			localname: "centos-8",
		},
		{
			image:     "registry.example.com:5000/foo/bar:baz",
			localname: "foo--bar-baz",
		},
		{
			image:     "registry.example.com:5000/foo/bar",
			localname: "foo--bar-latest",
		},
		{
			image:     "provision:latest",
			localname: "provision:latest",
		},
	}

	t.Parallel()
	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.image, func(t *testing.T) {
			actual := cmd.LocalImageName(tcase.image)
			assert.Equal(t, tcase.localname, actual)
		})
	}
}
