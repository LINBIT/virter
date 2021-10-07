# Importing vagrant boxes

Sometimes, Vagrant boxes are the only available sources of some VM. The following document describes the necessary steps
to import a Vagrant box into virter, using the example of Oracle Linux.

### 1. Locate the vagrant box url

You want to locate a `*.box` file to import. Make sure you are downloading the `libvirt` variant of the box.

Sometimes only a Vagrant "Repo" file is provided (likely ending in `.json`). The following command show how you can
extract the Vagrant Box url from such a "Repo", using Oracle Linux 8 as example:

```
$ curl -fsSL https://oracle.github.io/vagrant-projects/boxes/oraclelinux/8.json | jq '.versions[] | select(.providers | any(.name == "libvirt")) | { "version": .version, "url": .providers[].url}'
{
  "version": "8.4.257",
  "url": "https://yum.oracle.com/boxes/oraclelinux/ol8/OL8U4_x86_64-vagrant-libvirt-b257.box"
}
{
  "version": "8.4.221",
  "url": "https://yum.oracle.com/boxes/oraclelinux/ol8/OL8U4_x86_64-vagrant-libvirt-b221.box"
}
{
  "version": "8.3.198",
  "url": "https://yum.oracle.com/boxes/oraclelinux/ol8/OL8U3_x86_64-vagrant-libvirt-b198.box"
}
{
  "version": "8.3.183",
  "url": "https://yum.oracle.com/boxes/oraclelinux/ol8/OL8U3_x86_64-vagrant-libvirt-b183.box"
}
{
  "version": "8.2.145",
  "url": "https://yum.oracle.com/boxes/oraclelinux/ol8/ol8u2-libvirt-b145.box"
}
{
  "version": "8.2.134",
  "url": "https://yum.oracle.com/boxes/oraclelinux/ol8/ol8u2-libvirt-b134.box"
}
{
  "version": "8.2.125",
  "url": "https://yum.oracle.com/boxes/oraclelinux/ol8/ol8u2-libvirt-b125.box"
}
```

### 2. Load the qcow image

Now that you have located the `.box` URL to download, you can start the import in virter:

```
$ curl -fsSL https://yum.oracle.com/boxes/oraclelinux/ol8/OL8U4_x86_64-vagrant-libvirt-b257.box | tar xvzO box.img | virter image load vagrant-import
INFO[0000] loading from stdin
box.img
virter:work:load-vagrant compute digest done [=========================================================================================================================] 627.98MiB / 627.98MiB
virter:layer:sha256:06d4 buffer layer done   [=========================================================================================================================] 627.98MiB / 627.98MiB
virter:layer:sha256:06d4 upload layer done   [=========================================================================================================================] 627.98MiB / 627.98MiB
Loaded vagrant-import
```

### 3. Start the VM

Start the VM and connect to it using the vagrant provision key. Specify your desired `--name` for the image.

```
$ virter vm run --id 254 --name oracle-8 vagrant-import
INFO[0000] Create host key
INFO[0000] Create boot volume
INFO[0000] Create cloud-init volume
INFO[0000] Define VM
INFO[0000] Add DHCP entry from 52:54:00:00:00:fe to 192.168.122.254
INFO[0000] Start VM
$ curl -fsSL https://raw.githubusercontent.com/hashicorp/vagrant/main/keys/vagrant | ssh-add -
Identity added: (stdin) ((stdin))
$ ssh  -o "UserKnownHostsFile=/dev/null" -o "StrictHostKeyChecking=no" vagrant@192.168.122.254
Warning: Permanently added '192.168.122.254' (ED25519) to the list of known hosts.
Last login: Thu Oct  7 13:21:10 2021 from 192.168.122.1
[vagrant@localhost ~]$
```

### 4. Install cloud-init

How you install cloud init depends on your distribution. Use `apt`/`yum`/`dnf`/`whatever` to install.

```
[vagrant@localhost ~]$ sudo dnf install -y cloud-init
[vagrant@localhost ~]$ sudo systemctl enable --now cloud-init
[vagrant@localhost ~]$ exit
```

### 5. Shut down the VM and commit the changes

Now everything is prepared for virter, you just have to "save" the virtual machine:

```
$ virsh shutdown oracle-8
Domain 'oracle-8' is being shutdown

$ virter vm commit oracle-8
INFO[0000] Remove DHCP entry from 52:54:00:00:00:fe to 192.168.122.254
INFO[0000] Undefine VM
INFO[0000] deleted layer                                 layer="virter:work:oracle-8-cidata"
virter:work:oracle-8     compute digest done [=========================================================================================================================] 181.25MiB / 181.25MiB
virter:layer:sha256:7970 buffer layer done   [=========================================================================================================================] 181.25MiB / 181.25MiB
virter:layer:sha256:7970 upload layer done   [=========================================================================================================================] 181.25MiB / 181.25MiB
$ virter image rm vagrant-import
INFO[0000] deleted layer                                 layer="virter:tag:vagrant-import"
```

Now your VM image is ready to be used like any other virter image.
