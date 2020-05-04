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

The following provisioning types are supported.

## `docker`
A `docker` provisioning step allows specifying a Docker image to run provisioning steps in. This image will be executed on the host, so it will need to connect to the target VM and run its provisioning commands over SSH (or use a provisioning tool such as Ansible).

### Configuration Options
The Docker provisioning step can be parameterized using the following configuration options:
* `image` is the Docker image used to provision the VM. It follows the standard Docker format of `<repository>/<image>:<tag>`.
* `env` is a map of environment variables to be passed to the Docker container, in `KEY=value` format.

  Note that Virter already passes two environment variables by default:
  * `TARGETS` is a comma separated list of all VMs to run the provisioning on.
  * `SSH_PRIVATE_KEY` is the SSH private key Virter uses to connect to the machine as `root`.

## Shell
The `shell` provisioning step allows running arbitrary commands on the target VM over SSH. This is easier to use than the `docker` step, but also less flexible.

### Configuration Options
The `shell` provisioning step accepts the following parameters:
* `script` is a string containing the command(s) to be run.
  It can be either a single line string to run only a single command, or a multi-line string (as defined by toml), in which case every line of the string will be considered a separate command to run.
* `env` is a map of environment variables to be set in the target VM, in `KEY=value` format.

## rsync

The `rsync` provisioning step can be used to distribute files from the host to the guest machines using the `rsync` utility.

**NOTE**: This step requires that the `rsync` program is installed both on the host and on the guest machine. The user is responsible for making sure that this requirement is met.

### Configuration Options

* `source` is as a glob pattern of files on the host machine.
  It will first be expanded by Go's [os.ExpandEnv](https://golang.org/pkg/os/#ExpandEnv) and
  then interpreted according to the rules of Go's [filepath.Match](https://golang.org/pkg/path/filepath/#Match)
  function, so refer to the Go documentation for details.
* `dest` is the path on the guest machine(s) where the files should be copied to.

The glob-expanded `source` list of files and the `dest` path are passed verbatim to the `rsync` command line, so `rsync`'s path rules apply. Refer to the `rsync` documentation for more details.

## Global Options

There are also global options which can be set for all provisioning steps in a file.

* `env` is a map of environment variables in `KEY=value` format. These will be set in all provisioning steps that support `env` by themselves.

## Example
```
[env]
foo = "rck"

[[steps]]
[steps.docker]
image = "virter-hello-text:latest"
[steps.docker.env]
TEXT = "foo"
VAR_BAR = "hi"

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
