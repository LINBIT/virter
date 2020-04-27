#!/bin/sh
mkdir /root/.ssh
echo "$SSH_PRIVATE_KEY" > /root/.ssh/id_rsa
chmod 600 /root/.ssh/id_rsa
for h in $(echo "$TARGETS" | tr ',' '\n'); do
	/usr/bin/scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null /installer.sh "${h}:/tmp/installer.sh"
	/usr/bin/ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null $h "/tmp/installer.sh $KERNEL_VERSION && rm -f /tmp/installer.sh"
done
