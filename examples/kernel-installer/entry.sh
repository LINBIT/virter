#!/bin/sh
mkdir /root/.ssh
echo "$SSH_PRIVATE_KEY" > /root/.ssh/id_rsa
chmod 600 /root/.ssh/id_rsa
/usr/bin/scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null /installer.sh "${TARGET}:/tmp/installer.sh"
/usr/bin/ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null $TARGET "/tmp/installer.sh $KERNEL_VERSION && rm -f /tmp/installer.sh"
