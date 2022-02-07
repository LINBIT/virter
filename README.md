# Virter

Virter is a command line tool for simple creation and cloning of virtual
machines.

Virter supports VMs running standard general purpose distributions such as
CentOS and Ubuntu. It is especially useful for development and testing of
projects which cannot use containers due to kernel dependencies, such as
[DRBD](https://github.com/LINBIT/drbd) and
[LINSTOR](https://github.com/LINBIT/linstor-server).

## Quick Start

First install and set up [libvirt](https://libvirt.org/index.html). Then
download one of the [releases](https://github.com/LINBIT/virter/releases).
Virter is packaged as a single binary, so just put that into `/usr/local/bin` and
you are ready to use Virter:

```
virter image pull centos-7 # also would be auto-pulled in next step
virter vm run --name centos-7-hello --id 100 --wait-ssh centos-7
virter vm ssh centos-7-hello
virter vm rm centos-7-hello
```

To see a list of available images run the following:
```
virter image ls --available
```

Depending on your host distribution and libvirt installation, you may need to:
* [Set up a storage pool](#libvirt-storage-pool); and/or
* [Configure AppArmor](#apparmor).

## Usage

For usage just run `virter help`.

Some examples are provided in the [examples](./examples) directory. See the
README files for the individual examples.

There is also additional [documentation for the provisioning
feature](./doc/provisioning.md). This is useful for defining new images. These
images can be used to start clusters of cloned VMs.

## Installation Details

Virter requires:

* A running libvirt daemon on the host where it is run
* When container provisioning is used: A container runtime.
  Currently, Virter supports `docker` and `podman`.

Configuration is read by default from `~/.config/virter/virter.toml`.

When starting Virter for the first time, a default configuration file will be
generated, including documentation about the various flags.

### Container runtime

Select the container runtime by setting `container.provider` to either `docker` or `podman`.

#### podman

Virter communicates with `podman` via it's REST-API. Make sure the API socket is available.

This may be done by:

* Starting podman via systemd: `systemctl --user start podman.socket` (use `systemctl --user enable --now podman.socket` to make this permanent)
* Start podman manually: `podman system service`

### Network domain

If you require DNS resolution from your VMs to return correct FQDNs, add the
`domain` to your libvirt network definition:

```
<network>
  ...
  <domain name='test'/>
  ...
</network>
```

By default, Virter uses the libvirt network named `default`.

### DHCP Leases

Libvirt produces some weird behavior when MAC or IP addresses are reused while
there is still an active DHCP lease for them. This can result in a new VM
getting assigned a random IP instead of the IP corresponding to its ID.

To work around this, Virter tries to execute the `dhcp_release` utility in
order to release the DHCP lease from libvirt's DHCP server when a VM is
removed. This utility has to be run by the root user, so Virter executes
it using `sudo`.

If execution fails (for example because the utility is not installed or the
sudo rules are not set up correctly), the error is ignored by Virter.

So, to make Virter work more reliably, especially when you are running lots
of VMs in a short amount of time, you should install the `dhcp_release` utility
(usually packaged as `dnsmasq-utils`). Additionally, you should make sure that
your user can run `dhcp_release` as root, for example by using a sudo rule like
this:

```
%libvirt ALL=(ALL) NOPASSWD: /usr/bin/dhcp_release
```

This allows all users in the group libvirt to run the `dhcp_release` utility
without being prompted for a password.

### Console logs

The `--console` argument to `virter vm run` causes serial output from the VM to
be saved to a file. This file is created with the current user as the owner.
However, it is written to by libvirt, so it needs to located on a filesystem to
which libvirt can write. NFS mounts generally cannot be used due to
`root_squash`.

In addition, when the VM generates a lot of output, this can trigger `virtlogd`
to roll the log file over, which creates a file owned by root (assuming
`virtlogd` is running as root). To prevent this, increase `max_size` in
`/etc/libvirt/virtlogd.conf`.

### libvirt storage pool

Virter requires a libvirt storage pool for its images and VM volumes. By
default, it expects a pool named `default`. Some libvirt distributions
configure this pool automatically. Some, such as the standard packages on
Ubuntu Focal, do not. If the pool does not exist, create it like this:

```
virsh pool-define-as --name default --type dir --target /var/lib/libvirt/images
virsh pool-autostart default
virsh pool-start default
```

### AppArmor

On some distributions, AppArmor denies access to `/var/lib/libvirt/images` by default.
This leads to messages in dmesg along the lines of:

```
[ 4274.237593] audit: type=1400 audit(1598348576.161:102): apparmor="DENIED" operation="open" profile="libvirt-d84ef9d7-a7ad-4388-bd5d-cfc3a3db28a6" name="/var/lib/libvirt/images/centos-8" pid=14918 comm="qemu-system-x86" requested_mask="r" denied_mask="r" fsuid=64055 ouid=64055
```

This can be circumvented by overriding the AppArmor abstraction for that directory:

```
echo '/var/lib/libvirt/images/* rwk,' >> /etc/apparmor.d/local/abstractions/libvirt-qemu
systemctl restart apparmor.service
systemctl restart libvirtd.service
```

## Architecture

Virter connects to the libvirt daemon for all the heavy lifting. It supplies
bootstrap configuration to the VMs using `cloud-init` volumes, so that the
hostname is set and SSH access is possible.

## Building from source

If you want to test the latest unstable version of Virter, you can build the
git version from sources:

```
git clone https://github.com/LINBIT/virter
cd virter
go build .
```

## Comparison to other tools

### `virsh`

Virter is good for starting and cloning `cloud-init` based VMs. `virsh` is
useful for more detailed libvirt management. They work well together.

### `virt-install`

`virt-install` is built for images that use conventional installers. Virter
uses `cloud-init`, making it simpler to use and quicker to start a fresh VM.

### Running VMs in AWS/GCP/OpenNebula

Virter is local to a single host making snapshot/restore/clone operations very
efficient. Virter could be thought of as cloud provisioning for your local
machine.

### Vagrant

Virter and Vagrant have essentially the same goal. Virter is more tightly
integrated with the Linux virtualization stack, resulting in better
snapshot/restore/clone support.

### Multipass

Virter and Multipass have similar goals, but Multipass is Ubuntu specific.

### Docker

Virter is like Docker for VMs. The user experience of the tools is generally
similar. Docker containers share the host kernel, whereas Virter starts VMs
with their own kernel.

### Kata Containers

Virter starts VMs running a variety of Linux distributions, whereas Kata
Containers uses a specific guest that then runs containers.

### Weave Ignite

Ignite has very strong requirements on the guest, so it cannot be used for
running standard distributions.

## Development

Virter is a standard go project using modules.
Go 1.13+ is supported.
