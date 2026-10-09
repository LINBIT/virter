# Virter SSH Integration

The simplest way to connect to a virtual machine that was started with Virter
is to use `virter vm ssh ...`. However, you may also choose to use `ssh` and
related tools to connect. This can be made very convenient by adding the
following to your `~/.ssh/config`:

```
Match exec "virter vm exists %h"
    User root
    IdentityAgent none
    IdentityFile ~/.config/virter/id_rsa
    KnownHostsCommand /usr/bin/env virter vm host-key %n
```

Now you can easily connect:

```
$ ssh foo
[root@foo ~]#
```

## Name resolution

Depending on your configuration, `ssh` may or may not be able to resolve the VM
name to a hostname. If not, you will see an error similar to:

```
$ ssh foo
ssh: Could not resolve hostname foo: Name or service not known
```

Fix this by installing and configuring the [libvirt NSS
modules](https://libvirt.org/nss.html). In particular, you will need to install
a package such as `libvirt-nss` or `libnss-libvirt`. Then add `libvirt_guest`
to the `hosts:` configuration in the file `/etc/nsswitch.conf`.

## SSH integration with qualified names

If you have configured a network domain in your libvirt network, you can also
connect to the VM using the fully qualified domain name (FQDN). For instance,
with the domain name `test`, you can use this configuration in your
`~/.ssh/config`:

```
Host *.test
    User root
    IdentityAgent none
    IdentityFile ~/.config/virter/id_rsa
    KnownHostsCommand /bin/bash -c 'virter vm host-key "$(basename "%n" .test)"'
```

Now you can easily connect:

```
$ ssh foo.test
[root@foo ~]# 
```

## Host key check

Virter generates an SSH host key for each VM, passes it to the VM through
cloud-init, and accepts only that key. Guests that keep their own host keys,
such as Windows with [Cloudbase-Init](https://github.com/cloudbase/cloudbase-init),
need the `--ssh-trust-on-first-use` option on `vm run` or `image build`.
Virter then records the host key that the VM presents when it first gets
ready, during `vm run --wait-ssh`, `vm wait-ready` or `image build`, and
accepts only that key afterwards. Other commands, such as `vm exec` and
`vm ssh`, fail until then.
