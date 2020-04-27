#!/bin/sh
mkdir /root/.ssh
echo "$SSH_PRIVATE_KEY" > /root/.ssh/id_rsa
chmod 600 /root/.ssh/id_rsa
for h in $(echo "$TARGETS" | tr ',' '\n'); do
	/usr/bin/ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null $h "echo hello world > /file"
done
