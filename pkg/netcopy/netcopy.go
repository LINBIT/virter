package netcopy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// NetworkCopier copies files over the network
type NetworkCopier interface {
	// CopyTo transfers a list of files (source) to a given directory (destination).
	// Any one of the paths may be located on the host or remotely
	Copy(ctx context.Context, source []HostPath, destination HostPath) error
}

// The default copier. Uses `rsync` to do the actual work
type RsyncNetworkCopier struct {
	sshPrivateKeyPath string
}

// HostPath stores a path with host information
type HostPath struct {
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

func NewRsyncNetworkCopier(sshPrivateKeyPath string) *RsyncNetworkCopier {
	return &RsyncNetworkCopier{
		sshPrivateKeyPath,
	}
}

func (r *RsyncNetworkCopier) Copy(ctx context.Context, sources []HostPath, dest HostPath) error {
	if len(sources) == 0 {
		log.Debugf("got empty sources, nothing to copy. %v -> %v", sources, dest)
		return nil
	}

	args := []string{"--recursive", "--perms", "--times"}

	for _, src := range sources {
		args = append(args, formatRsyncArg(src))
	}

	args = append(args, formatRsyncArg(dest))

	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Env = []string{
		// TODO: we are ignoring the SSH host key here. ideally we would
		// somehow get the host key beforehand and properly verify them.
		fmt.Sprintf(`RSYNC_RSH=/usr/bin/ssh -i "%s" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null`, r.sshPrivateKeyPath),
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

	return fmt.Sprintf("root@%s:%s", spec.Host, spec.Path)
}
