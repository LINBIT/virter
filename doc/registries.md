# Virter Image Registries

In order to know where to look when pulling VM images, virter uses a mechanism
called an "image registry" to map an image's name to an URL where the VM image
can be downloaded.

## File structure

Image registry files are simple enough in principle:

```toml
[ubuntu-focal]
url = "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img"
```

A registry file is just a [toml](https://github.com/toml-lang/toml) file, with
each section corresponding to an image and a `url` key to specify the VM image
location.

## Locations

Virter tries to load its image registry from two locations:

* `$XDG_DATA_HOME/virter` (defaults to `$HOME/.local/share/virter`). This is the
  "shipped" image registry.
* `$CONFIG_DIR/images.toml` where `$CONFIG_DIR` is defined as the path where
  virters configuration file (`virter.toml`) is stored. This is the "user-defined"
  image registry.

### Shipped Registry

The shipped image registry file is pre-populated with some sensible default images,
which represents some commonly used distributions. It is maintained by the
virter maintainers.

If the shipped image registry file does not exist, it is fetched from a well-known
static url (https://linbit.github.io/virter/images.toml). The shipped registry
can also be updated manually, using the `virter registry update` command.

### User-Defined Registry

The user-defined image registry file resides next to `virter.toml`, virters
configuration file. It usually does not exist, but can be created by the user
to define additional images or override images from the shipped registry.

When virter loads its image registry, it first loads the shipped registry file.
Then, it loads the user-defined registry file. When the name of an image collides
with an already defined name from the shipped registry, it is overridden.

The combined contents of the image registries can be viewed by using
`virter image ls --available`.
