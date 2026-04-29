# Migrating from Virter 0.x to 1.0

This document describes the breaking changes between Virter 0.x and 1.0, and
how to update existing configuration files, provisioning files, and scripts.

## Image registry file format

Registry files now require a `version` field and nest image entries under an
`[images]` table.

**Before (0.x):**

```toml
[ubuntu-noble]
url = "https://example.com/ubuntu-noble.qcow2"
```

**After (1.0):**

```toml
version = 1

[images.ubuntu-noble]
url = "https://example.com/ubuntu-noble.qcow2"
```

Loading a registry file without the correct version will fail with
`unsupported registry file version`.

The upstream registry URL has also changed and is now versioned. Virter 1.0
fetches from `https://linbit.github.io/virter/v1/images.toml`. The old
unversioned URL (`/virter/images.toml`) continues to serve the 0.x format and
will not pick up new images.

## Provisioning files

### Strict key checking

Unknown keys in provisioning files used to produce a warning and were
otherwise ignored. In 1.0 they are a hard error. This catches typos like
`destination` instead of `dest` early.

If you see `unknown keys in provisioning file: ...`, remove or correct the
listed keys.

### Filesystem access restricted to the working directory

`rsync.source` and `container.copy.dest` may now only refer to paths *inside*
the current working directory of Virter, analogous to how `docker build`
restricts `COPY`/`ADD` to the build context.

Provisioning files that read from or write to absolute paths outside the cwd
(or escape it via `..`) will fail with
`... not allowed: ... outside working directory`.

To migrate, move the referenced files into the working directory (or run
Virter from a directory that contains them).

## CLI Flags

### Removed: `--pull-policy`

`--pull-policy` on `vm run` and `image build` has been removed. Use
`--vm-pull-policy` instead, which has been the recommended replacement
throughout 0.x.

```sh
# 0.x
virter vm run --pull-policy IfNotExist ...

# 1.0
virter vm run --vm-pull-policy IfNotExist ...
```

### Renamed: `--boot-capacity`

The old flag names are inconsistent with the rest of the CLI:

* `vm run --bootcapacity` → `vm run --boot-capacity`
* `image build --bootcap` → `image build --boot-capacity`

The old names still work in 1.0 but are deprecated and will be removed in a
future release. Update scripts now to silence the deprecation warnings.
