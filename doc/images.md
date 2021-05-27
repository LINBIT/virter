# Virter Images

Virter images are the templates for root volumes of VMs. Images consist of one or more "layers" (actually libvirt
volumes) in QCOW2 format. QCOW2 allows for overlays, i.e. every layer only stores the diff to the previous layer.

This makes starting VMs a cheap operation: for every VM, virter creates another empty layer, referencing the common
image layers. The empty layer will grow to store the changes during the VMs execution. 

## Sharing images

Virter images can be shared via container registries or pulled from a predefined list of HTTP servers. 

## Pulling images

To pull the image `my.registry.com/my-namespace/my-image:tag` to a virter image named `local-image`, use:

```
$ virter image pull local-image my.registry.com/my-namespace/my-image:tag
sha256:0f1a974bbae614165 pull done [=================] 282.82MiB / 282.82MiB
sha256:99bd66cc35a3db143 pull done [=====================] 1.93MiB / 1.93MiB
Pulled local-image
```

Pulling from a container registry will always update the local image. If the image was already present, but in an older
version, it will be replaced. If a VM was started using the old image, removal of the old image is delayed until
all VMs are removed.

If your registry requires authentication, use your local `docker` or `podman` command to log in to the registry

```
docker login my.registry.com
```

Instead of a container registry, you can also pull images from HTTP resources. You can either specify a URL directly:

```
$ virter image pull ubuntu-bionic https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img
ubuntu-bionic pull done [===============================================] 353.56MiB / 353.56MiB
Pulled ubuntu-bionic
```

or use the [Virter Image Registry](images.md#virter-image-registry) to select the HTTP resource to pull:

```
$ virter image pull ubuntu-bionic
ubuntu-bionic pull done [===============================================] 353.56MiB / 353.56MiB
Pulled ubuntu-bionic
```

## Pushing images

Virter can push images to container registries:

```
$ virter image push local-image my.registry.com/my-namespace/my-image:tag
sha256:99bd66cc35a3db143 compress done [==================================] 13.06MiB / 13.06MiB
sha256:0f1a974bbae614165 compress done [================================] 284.94MiB / 284.94MiB
Pushed my.registry.com/my-namespace/my-image:tag
```

## Save and load images from the local filesystem

You can save an image to a file:

```
$ virter image save local-image my-img.qcow2
local-image save done [================================================] 867.69MiB / 867.69MiB
Saved local-image
```

To load it again:

```
$ virter image load local-image my-img.qcow2
local-image load done           [======================================] 867.69MiB / 867.69MiB
virter:work:load-local-image compute digest done [=====================] 867.69MiB / 867.69MiB
Loaded local-image
```

## Virter Image Registry

In order to know where to look when pulling VM images, virter uses a mechanism
called an "image registry" to map an image's name to a URL where the VM image
can be downloaded.

### File structure

Image registry files are simple enough in principle:

```toml
[ubuntu-focal]
url = "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img"
```

A registry file is just a [toml](https://github.com/toml-lang/toml) file, with
each section corresponding to an image and a `url` key to specify the VM image
location.

### Locations

Virter tries to load its image registry from two locations:

* `$XDG_DATA_HOME/virter` (defaults to `$HOME/.local/share/virter`). This is the
  "shipped" image registry.
* `$CONFIG_DIR/images.toml` where `$CONFIG_DIR` is defined as the path where
  virters configuration file (`virter.toml`) is stored. This is the "user-defined"
  image registry.

#### Shipped Registry

The shipped image registry file is pre-populated with some sensible default images,
which represents some commonly used distributions. It is maintained by the
virter maintainers.

If the shipped image registry file does not exist, it is fetched from a well-known
static url (https://linbit.github.io/virter/images.toml). The shipped registry
can also be updated manually, using the `virter registry update` command.

#### User-Defined Registry

The user-defined image registry file resides next to `virter.toml`, virters
configuration file. It usually does not exist, but can be created by the user
to define additional images or override images from the shipped registry.

When virter loads its image registry, it first loads the shipped registry file.
Then, it loads the user-defined registry file. When the name of an image collides
with an already defined name from the shipped registry, it is overridden.

The combined contents of the image registries can be viewed by using
`virter image ls --available`.
