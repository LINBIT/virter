#!/bin/sh
mkdir /root/.ssh
echo "$SSH_PRIVATE_KEY" > /root/.ssh/id_rsa
chmod 600 /root/.ssh/id_rsa
/usr/bin/ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null $TARGET "echo hello world > /file"
