package virter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeHostKey struct{}

func (fakeHostKey) PrivateKey() string { return "PRIVATE\nKEY" }
func (fakeHostKey) PublicKey() string  { return "ssh-rsa HOSTKEY" }

func TestRenderUserData(t *testing.T) {
	base := userDataConfig{
		VMName:        "vm1",
		SSHPublicKeys: []string{"ssh-rsa USERKEY"},
		HostKey:       fakeHostKey{},
	}

	t.Run("without-apt-mirror", func(t *testing.T) {
		actual, err := renderUserData(base)
		assert.NoError(t, err)
		assert.True(t, strings.HasPrefix(actual, "#cloud-config\n"), actual)
		assert.NotContains(t, actual, "jinja")
		assert.NotContains(t, actual, "apt:")
	})

	t.Run("with-apt-mirror", func(t *testing.T) {
		cfg := base
		cfg.AptMirror = "https://mirror.example.com/"
		actual, err := renderUserData(cfg)
		assert.NoError(t, err)
		assert.True(t, strings.HasPrefix(actual, "## template: jinja\n#cloud-config\n"), actual)

		expectedApt := `
apt:
  primary:
    - arches: [default]
      uri: https://mirror.example.com/{{ v1.distro }}
  security:
    - arches: [default]
      uri: https://mirror.example.com/{{ v1.distro }}{% if v1.distro == 'debian' %}-security{% endif %}
`
		assert.True(t, strings.HasSuffix(actual, expectedApt), actual)
	})
}
