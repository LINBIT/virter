package virter

import (
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHostKey struct{}

func (fakeHostKey) PrivateKey() string { return "PRIVATE\nKEY" }
func (fakeHostKey) PublicKey() string  { return "ssh-rsa HOSTKEY" }

func TestMetaData(t *testing.T) {
	keys := []string{
		"ssh-ed25519 USERKEY work: laptop #2",
		"ssh-rsa VIRTERKEY\n",
	}
	var v *Virter
	actual, err := v.metaData("vm1", keys)
	assert.NoError(t, err)

	expected := `instance-id: vm1
local-hostname: vm1
public-keys:
  - "ssh-ed25519 USERKEY work: laptop #2"
  - "ssh-rsa VIRTERKEY"
`
	assert.Equal(t, expected, actual)
}

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

		// cloud-init parses user-data with Python's email parser, so
		// parse it as a mail message here as well.
		msg, err := mail.ReadMessage(strings.NewReader(actual))
		require.NoError(t, err)
		mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "multipart/mixed", mediaType)

		type part struct {
			contentType string
			content     string
		}
		var parts []part
		r := multipart.NewReader(msg.Body, params["boundary"])
		for {
			p, err := r.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			content, err := io.ReadAll(p)
			require.NoError(t, err)
			ct, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
			require.NoError(t, err)
			parts = append(parts, part{contentType: ct, content: string(content)})
		}
		require.Len(t, parts, 2)

		// The base configuration must not depend on jinja, so that
		// cloud-init versions without jinja support still apply it.
		withoutMirror, err := renderUserData(base)
		require.NoError(t, err)
		assert.Equal(t, "text/cloud-config", parts[0].contentType)
		assert.Equal(t, withoutMirror, parts[0].content)
		assert.NotContains(t, parts[0].content, "jinja")

		expectedApt := `## template: jinja
#cloud-config
apt:
  primary:
    - arches: [default]
      uri: https://mirror.example.com/{{ v1.distro }}
  security:
    - arches: [default]
      uri: https://mirror.example.com/{{ v1.distro }}{% if v1.distro == 'debian' %}-security{% endif %}
`
		assert.Equal(t, "text/jinja2", parts[1].contentType)
		assert.Equal(t, expectedApt, parts[1].content)
	})
}
