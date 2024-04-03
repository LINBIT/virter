# Provisioning
Virter supports running provisioning steps either on running VMs or to build a new VM image from an existing one.

Conceptually, this is similar to using `docker exec` on a running container and using a Dockerfile to build a Docker image, respectively.

To define the provisioning process, Virter uses files in the [toml](https://github.com/toml-lang/toml) format. In such a file, each provisioning step can be specified, and Virter will execute them in order.

A provisioning file can be used when building a VM image as such:
```sh
$ virter image build -p provisioning.toml centos7 centos7-provisioned
```
This will create a new image – `centos7-provisioned` – which is derived from the `centos7` base image by applying the provisioning steps defined in `provisioning.toml`.

It can also be applied to one or multiple already running VMs:
```sh
$ virter vm exec -p provisioning.toml centos-1 centos-2 centos-3
```

If a container or shell provisioning step fails, the virter process will exit with the same exit code as the provisioning script.

## Provisioning types

The following provisioning types are supported.

### `container`
A `container` provisioning step allows specifying a container image to run provisioning steps in.
This image will be executed on the host using the provider configured in `container.provider` (such as Docker or Podman).
It will need to connect to the target VM and run its provisioning commands over SSH (or use a provisioning tool such as Ansible).

The container provisioning step can be parameterized using the following configuration options:
* `image` is the container image used to provision the VM. It follows the standard format of `<repository>/<image>:<tag>`. This is a Go template.
* `pull` specifies when the above image should be pulled. Valid values are `Always`, `IfNotExist` or `Never`. If not specified, the default is `IfNotExist`. Can be overridden during execution using `--container-pull-policy`.
* `env` is a map of environment variables to be passed to the container, in `KEY=value` format. The values are Go templates.

  Note that Virter already passes these environment variables by default:
  * `TARGETS` is a comma separated list of all VMs to run the provisioning on.
  * `SSH_PRIVATE_KEY` is the SSH private key Virter uses to connect to the machine as `root`.
  * `VIRTER_ACCESS_NETWORK` is the network interface that virter is using to  connect to the VMs. It is provided in CIDR notation, with the address being the address of the host. (Example: `192.168.122.1/24`)
  

* `command` is a string array and sets the command to execute in the container (basically `<args>...` in `docker run <image> <args>...`). The items are Go templates.
* `copy` can be used to retrieve files from the container after the provisioning has finished. `source` is the file or directory within the container to copy out, and `dest` is the path on the host where the file or directory should be copied to. The `dest` value is a Go template.

In addition, every container binds the following paths:
* The current working directory of Virter, exposed read only at `/virter/workspace`
* The SSH private key Virter used to connect to the machine as root at `/root/.ssh/id_rsa`
* The SSH known hosts file, prefilled for connecting to the machine at `/root/.ssh/known_hosts`
* A SSH config file that contains a mapping from VMs to user names to be used for ssh connections (to support platforms where the "root" user does not exist). This file is mapped under `/etc/ssh/ssh_config.virter`. Use `ssh -F /etc/ssh/ssh_config.virter <ip-address-or-hostname>` to use this file (without specifying user name explicitly).

### Shell
The `shell` provisioning step allows running arbitrary commands on the target VM over SSH. This is easier to use than the `container` step, but also less flexible.

The `shell` provisioning step accepts the following parameters:
* `script` is a string containing the command(s) to be run.
  It can be either a single line string to run only a single command, or a multi-line string (as defined by toml), in which case every line of the string will be considered a separate command to run.
* `env` is a map of environment variables to be set in the target VM, in `KEY=value` format. The values are Go templates.

### rsync

The `rsync` provisioning step can be used to distribute files from the host to the guest machines using the `rsync` utility.

**NOTE**: This step requires that the `rsync` program is installed both on the host and on the guest machine. The user is responsible for making sure that this requirement is met.

The `rsync` provisioning step accepts the following parameters:
* `source` is as a glob pattern of files on the host machine.
  It will first be expanded as a Go template and
  then interpreted according to the rules of Go's [filepath.Match](https://golang.org/pkg/path/filepath/#Match)
  function, so refer to the Go documentation for details.
* `dest` is the path on the guest machine(s) where the files should be copied to.

The glob-expanded `source` list of files and the `dest` path are passed verbatim to the `rsync` command line, so `rsync`'s path rules apply. Refer to the `rsync` documentation for more details.

## Global Options

There are also global options which can be set for all provisioning steps in a file.

* `env` is a map of environment variables in `KEY=value` format. These will be set in all provisioning steps that support `env` by themselves. The values are Go templates.

## Template values

As documented for the various provisioning types above, many of the values in a provisioning file are interpreted as
[Go templates](https://golang.org/pkg/text/template/).

The data provided to these templates is the `[values]` section in the provisioning file. These values can be set or overridden with `--set`. For instance:
```
$ virter vm exec my-vm -p examples/hello-world/hello-world.toml --set values.Image=my-image-name
```

## Example
```
version = 1

[values]
Image = "virter-hello-text"

[env]
foo = "rck"

[[steps]]
[steps.container]
# Go templating can be used for many values
image = "{{.Image}}"
[steps.container.env]
TEXT = "foo"
VAR_BAR = "hi"
[steps.container.copy]
source = "/tmp/somefile"
dest = "."

[[steps]]
[steps.shell]
script = "env && touch /tmp/$foo"
[steps.shell.env]
# this overrides the global env variable foo
foo = "rck"

[[steps]]
[steps.shell]
script = """
yum remove -y make
yum install -y make
"""

[[steps]]
[steps.rsync]
source = "/tmp/*.rpm"
dest = "/root/rpms"
```

## Setting/overriding configuration steps on the command line

It is possible to define provisioning steps entirely on the command line as one
might know from [helm](https://helm.sh). This is useful for very simple
provisioning:

```shell
$ virter vm exec --set steps[0].shell.script=env centos-1 centos-2 centos-3
```

This is especially useful when combined with a provisioning configuration file to override variables like this:

```shell
$ virter vm exec -p provisioning.toml --set env.foo=bar centos-1 centos-2 centos-3
```

## Caching provision images

You can directly push your provision image to a registry using the `--push` option:

```
$ virter image build ubuntu-focal registry.example.com/my-image:latest --push
```

You can skip rebuilding the same image every time you run `virter image build` by specifying a `--build-id` when using
`--push`:

```
$ virter image build ubuntu-focal registry.example.com/my-image:latest --push --build-id my-latest-build
```

The build ID acts as a cache key: as long the build ID is the same for every `virter image build` command, virter
will re-use the previously provisioned image. If the build ID changes or the current build would use a different
base image virter will re-run the provisioning, even if the build ID remains the same.

For example, if you only want to rerun provisioning if the provisioning config was changed, you can run
```
$ virter image build ubuntu-focal registry.example.com/my-image:latest --push --build-id $(sha256sum provision.toml) -p provision.toml
$ # Alternatively, if you are running in a git repository, you could use:
$ virter image build ubuntu-focal registry.example.com/my-image:latest --push --build-id $(git rev-list -1 HEAD -- provision.toml) -p provision.toml
```

To always rebuild the image, even if using `--build-id`, use the `--no-cache` flag.
