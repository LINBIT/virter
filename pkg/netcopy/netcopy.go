package netcopy

import (
	"fmt"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

type RsyncNetworkCopier struct {
	sshPrivateKeyPath string
}

func NewRsyncNetworkCopier(sshPrivateKeyPath string) *RsyncNetworkCopier {
	return &RsyncNetworkCopier{
		sshPrivateKeyPath,
	}
}

func (r *RsyncNetworkCopier) Copy(host string, source []string, dest string) error {
	destPath := fmt.Sprintf("root@%s:%s", host, dest)
	args := append(source, destPath)

	cmd := exec.Command("rsync", args...)
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
