# Configuration

Virter is configured through a TOML file. The default location is
`~/.config/virter/virter.toml` (or `$XDG_CONFIG_HOME/virter/virter.toml` if
that environment variable is set). When Virter starts and no config file
exists, a default one is generated with inline documentation for each option.

A different config file can be selected with the `--config` flag.

## Setting values

A given config key can be set in the following ways. From lowest to highest
priority:

1. **Config file.** The TOML file at the default location, or the path
   given to `--config`.
2. **Environment variable.** Each key has a corresponding environment
   variable: prefix with `VIRTER_` and replace `.` with `_`, all uppercase.
   For example, `libvirt.pool` is set with `VIRTER_LIBVIRT_POOL`.
3. **`--config-set` flag.** A flag on the root command that takes `key=value`.
   For example, to run a VM using a non-default pool without changing the
   config file:

   ```
   virter --config-set libvirt.pool=fast-ssd vm run --id 10 --name test alma-10
   ```

   `--config-set` may be repeated for different keys. If the same key is
   given more than once, the last value wins. List-valued keys (such as
   `auth.user_public_key` or `libvirt.dnsmasq_options`) cannot be set with
   `--config-set`; use the config file instead.

## Available keys

See the generated default config file for the full list of keys, their
default values, and a description of each.
