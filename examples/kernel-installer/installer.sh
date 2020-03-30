#!/bin/bash

kernel::rpm() {
	local k=$1
	rpm -qa | grep '^kernel-[0-9]\+' | sort > /tmp/had
	yum install -y "$k"
	rpm -qa | grep '^kernel-[0-9]\+' | sort > /tmp/have

	for k in $(comm -12 /tmp/had /tmp/have); do
		rpm -e $k # yum autoremove does not like to remove the running kernel
	done
}

kernel::deb() {
	local k=$1
	dpkg-query -f '${Package}\n' -W "linux-image-*" | grep 'linux-image-[0-9]\+' | sort > /tmp/had
	apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y "$k"
	dpkg-query -f '${Package}\n' -W "linux-image-*" | grep 'linux-image-[0-9]\+' | sort > /tmp/have

	for k in $(comm -12 /tmp/had /tmp/have); do
		DEBIAN_FRONTEND=noninteractive apt-get autoremove -y $k
	done
}

fmt=rpm
[ -n "$(type -p dpkg)" ] && fmt=deb
kernel::$fmt $1
rm -f /tmp/ha{d,ve}
