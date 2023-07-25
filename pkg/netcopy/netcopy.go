package netcopy

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/virter/pkg/sshkeys"
)

// NetworkCopier copies files over the network
type NetworkCopier interface {
	// Copy transfers a list of files (source) to a given directory (destination).
	// Any one of the paths may be located on the host or remotely.
	Copy(ctx context.Context, source []HostPath, destination HostPath, keyStore sshkeys.KeyStore, knownHosts sshkeys.KnownHosts) error
}

// The default copier. Uses `rsync` to do the actual work
type RsyncNetworkCopier struct{}

// HostPath stores a path with host information
type HostPath struct {
	User string
	Path string
	Host string
}

// Parse a host path from a string in '[HOST:]PATH' form.
// no 'HOST:' part implies the local machine
func ParseHostPath(v string) HostPath {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) == 1 {
		return HostPath{
			Path: v,
		}
	}

	// Note: we may need to support path names with ":" in it. If the side before the ":"
	// contains any slashes, it must be a path.
	if strings.Contains(parts[0], "/") {
		return HostPath{
			Path: v,
		}
	}

	return HostPath{
		Host: parts[0],
		Path: parts[1],
	}
}

func (h *HostPath) Local() bool {
	return h.Host == ""
}

func NewRsyncNetworkCopier() *RsyncNetworkCopier {
	return &RsyncNetworkCopier{}
}

func (r *RsyncNetworkCopier) Copy(ctx context.Context, sources []HostPath, dest HostPath, keyStore sshkeys.KeyStore, knownHosts sshkeys.KnownHosts) error {
	if len(sources) == 0 {
		log.Debugf("got empty sources, nothing to copy. %v -> %v", sources, dest)
		return nil
	}

	knownHostsFile, err := ioutil.TempFile("", "rsync-known-hosts-*")
	if err != nil {
		return fmt.Errorf("failed to create known hosts file: %w", err)
	}

	defer os.Remove(knownHostsFile.Name())

	err = knownHosts.AsKnownHostsFile(knownHostsFile)
	if err != nil {
		return fmt.Errorf("failed to write known hosts file: %w", err)
	}

	err = knownHostsFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close known hosts file: %w", err)
	}

	args := []string{"--recursive", "--perms", "--times", "--protect-args"}

	for _, src := range sources {
		args = append(args, formatRsyncArg(src))
	}

	args = append(args, formatRsyncArg(dest))

	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Env = []string{
		fmt.Sprintf(`RSYNC_RSH=ssh -i "%s" -o UserKnownHostsFile=%s -o PubkeyAcceptedKeyTypes=+ssh-rsa`, keyStore.KeyPath(), knownHostsFile.Name()),
	}

	log.Debugf("executing rsync command:")
	log.Debugf("%s %s %s", strings.Join(cmd.Env, " "), cmd.Path, strings.Join(cmd.Args[1:], " "))

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Debugf("rsync output:\n%s", string(out))
		return fmt.Errorf("error executing rsync: %w", err)
	}
	return nil
}

func formatRsyncArg(spec HostPath) string {
	if spec.Host == "" {
		return spec.Path
	}

	return fmt.Sprintf("%s@%s:%s", spec.User, spec.Host, spec.Path)
}
