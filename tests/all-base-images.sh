#!/usr/bin/env bash
# Generate a vms.toml for vmshed, using known base images.
set -e

cat <<EOF
name = "smoke"
provision_file = "provision.toml"
provision_timeout = "2m"
provision_memory = "1G"
provision_cpus = 1
EOF

while read -r name url; do
	if echo "$name" | grep -q -E "^($EXCLUDED_BASE_IMAGES)\$" ; then
		echo "$name was excluded" >&2
		continue
	fi

	if [ -n "$PULL_LOCATION" ]; then
		virter image pull "$name" "$PULL_LOCATION/$name" >&2
	else
		virter image pull "$name" >&2
	fi

	cat <<EOF

[[vms]]
vcpus = 1
memory = "1G"
base_image = "$name"
EOF

done < <(virter image ls --available | tail -n +2)
