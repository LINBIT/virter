# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `cloudinit.apt_mirror` config option to point apt in Debian and Ubuntu VMs at
  a local mirror. Requires cloud-init 18.4 or newer in the guest image.

## [1.3.2] - 2026-09-10

### Fixed

- Shared disks and raw extra disks are created as sparse volumes instead of
  being fully allocated up front.

## [1.3.1] - 2026-09-08

### Changed

- Disk I/O errors are reported to the guest instead of pausing the whole VM,
  so a full host disk no longer looks like a silent stall.

### Fixed

- `vm run --id` reclaims a DHCP reservation left behind by an interrupted
  `vm rm` instead of failing with "preset ID already used".
- `image build` no longer panics when interrupted during the commit phase.

## [1.3.0] - 2026-08-25

### Added

- `image build --push-to` flag to push to a registry reference while keeping a
  separate local image name.

### Fixed

- `image build --push` rejects destinations without a registry early instead
  of failing later with an authentication error.

## [1.2.1] - 2026-08-13

### Fixed

- `disk ls` and `disk rm` no longer fail when a VM is removed concurrently.

## [1.2.0] - 2026-08-12

### Added

- `virter disk` command group (`create`, `ls`, `rm`) to manage shared disks
  that can be attached to multiple VMs at once.
- `vm run --shared-disk` flag to attach a shared disk. Shared disks survive
  `vm rm` and cannot be removed while attached.
- `--config-set key=value` flag on all commands to override config file and
  environment variable settings.

## [1.1.0] - 2026-05-26

### Added

- `user` field on the `container` provisioning step to override the user the
  container command runs as.

## [1.0.0] - 2026-04-30

See [doc/migration-1.0.md](doc/migration-1.0.md) for the full 0.x to 1.0
migration guide.

### Added

- `vm list --sort` flag to sort by `name`, `id`, `network`, or `state`. Output
  is sorted by name by default.

### Changed

- Image pulls request identity encoding, avoiding needless gzip of already
  compressed disk images.
- `vm rm` no longer logs every deleted layer; image commands log a single
  summary line instead.

### Removed

- Deprecated `--bootcapacity` and `--bootcap` flags. Use `--boot-capacity`.

### Fixed

- `auth.user_public_key` accepts a single string again instead of splitting it
  on whitespace.
- Progress bar no longer hangs when the server does not send a Content-Length.

[Unreleased]: https://github.com/LINBIT/virter/compare/v1.3.2...HEAD
[1.3.2]: https://github.com/LINBIT/virter/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/LINBIT/virter/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/LINBIT/virter/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/LINBIT/virter/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/LINBIT/virter/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/LINBIT/virter/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/LINBIT/virter/compare/v0.30.0...v1.0.0
