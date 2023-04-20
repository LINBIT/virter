package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/pullpolicy"
	"github.com/LINBIT/virter/pkg/sshkeys"
)

// defaultConfigTemplate contains a template for virter's default config file.
// It is intended to be used with a template.FuncMap which maps the "get" function
// to viper.Get. This way, the default values will be represented in the default
// configuration file.
// The default config file also contains some inline documentation for each option.
const defaultConfigTemplate = `[libvirt]
# pool is the libvirt pool that virter should use.
# The user is responsible for ensuring that this pool exists and is active.
# Default value: "{{ get "libvirt.pool" }}"
pool = "{{ get "libvirt.pool" }}"

# network is the libvirt network that virter should use.
# The user is responsible for ensuring that this network exists and is active.
# Default value: "{{ get "libvirt.network" }}"
network = "{{ get "libvirt.network" }}"

# static_dhcp is a boolean flag that determines how virter handles DHCP hosts
# entries. When false, the hosts entries are added and removed as needed.
# When true, the hosts entries must be created (with 'virter network host add')
# before the VM is started. Adding and removing DHCP entries can temporarily
# disrupt network access from running VMs. With this option, that disruption
# can be controlled.
# Default value: "{{ get "libvirt.static_dhcp" }}"
static_dhcp = "{{ get "libvirt.static_dhcp" }}"

# dnsmasq_options is an array of dnsmasq options passed when creating a new
# network. Options are strings, corresponding to the long flags described
# here: https://dnsmasq.org/docs/dnsmasq-man.html
# Default value: {{ get "libvirt.dnsmasq_options" }}
dnsmasq_options = {{ get "libvirt.dnsmasq_options" }}

# disk_cache is passed to libvirt as the disk driver cache attribute. See
# https://libvirt.org/formatdomain.html#hard-drives-floppy-disks-cdroms.
# Default value: "{{ get "libvirt.disk_cache" }}"
disk_cache = "{{ get "libvirt.disk_cache" }}"

[time]
# ssh_ping_count is the number of times virter will try to connect to a VM's
# ssh port after starting it.
# Default value: {{ get "time.ssh_ping_count" }}
ssh_ping_count = {{ get "time.ssh_ping_count" }}

# ssh_ping_period is how long virter will wait between to attempts at trying
# to reach a VM's ssh port after starting it.
# Default value: "{{ get "time.ssh_ping_period" }}"
ssh_ping_period = "{{ get "time.ssh_ping_period" }}"

# shutdown_timeout is how long virter will wait for a VM to shut down.
# If a shutdown operation exceeds this timeout, an error will be produced.
# Default value: "{{ get "time.shutdown_timeout" }}"
shutdown_timeout = "{{ get "time.shutdown_timeout" }}"

[auth]
# virter_public_key_path is where virter should place its generated public key.
# If this file does not exist and the file from virter_private_key_path exists,
# an error will be produced.
# If neither of the files exist, a new keypair will be generated and placed at the
# respective locations.
# If both files exist, they will be used to connect to all VMs started by virter.
# Default value: "{{ get "auth.virter_public_key_path" }}"
virter_public_key_path = "{{ get "auth.virter_public_key_path" }}"

# virter_private_key_path is where virter should place its generated private key.
# If this file does not exist and the file from virter_public_key_path exists,
# an error will be produced.
# If neither of the files exist, a new keypair will be generated and placed at the
# respective locations.
# If both files exist, they will be used to connect to all VMs started by virter.
# Default value: "{{ get "auth.virter_private_key_path" }}"
virter_private_key_path = "{{ get "auth.virter_private_key_path" }}"

# user_public_key can be used to define additional public keys to inject into
# the VM. If non-empty, the contents of this string will be added to the root
# user's authorized keys inside the VM.
# Default value: "{{ get "auth.user_public_key" }}"
user_public_key = "{{ get "auth.user_public_key" }}"

[container]
# provider is the container engine used. Can be either podman or docker.
provider = "{{ get "container.provider" }}"

# default pull policy to apply if non was specified. Can be 'Always', 'IfNotExist' or 'Never'.
# Default value: "{{ get "container.pull" }}"
pull = "{{ get "container.pull" }}"
`

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetDefault("libvirt.pool", "default")
	viper.SetDefault("libvirt.network", "default")
	viper.SetDefault("libvirt.static_dhcp", false)
	viper.SetDefault("libvirt.dnsmasq_options", []string{})
	viper.SetDefault("libvirt.disk_cache", "")
	viper.SetDefault("time.ssh_ping_count", 300)
	viper.SetDefault("time.ssh_ping_period", time.Second)
	viper.SetDefault("time.shutdown_timeout", 20*time.Second)
	viper.SetDefault("auth.user_public_key", "")
	viper.SetDefault("container.provider", "docker")
	viper.SetDefault("container.pull", "IfNotExist")

	viper.SetConfigType("toml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)

		// When using a custom config file, do not set a default key location
		viper.SetDefault("auth.virter_public_key_path", "")
		viper.SetDefault("auth.virter_private_key_path", "")
	} else {
		p := configPath()
		if err := os.MkdirAll(p, 0700); err != nil {
			log.Fatalf("Could not create directory %q: %v", p, err)
		}
		// Use config file from standard location
		viper.AddConfigPath(p)
		viper.SetConfigName("virter")

		// When using the default config file location, make that also the default key location
		viper.SetDefault("auth.virter_public_key_path", filepath.Join(p, "id_rsa.pub"))
		viper.SetDefault("auth.virter_private_key_path", filepath.Join(p, "id_rsa"))
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Debugf("Using config file: %s", viper.ConfigFileUsed())
	} else if isConfigNotFoundError(err) {
		newPath := cfgFile
		if newPath == "" {
			newPath = filepath.Join(configPath(), "virter.toml")
		}
		log.Print("Config file does not exist, creating default: ", newPath)

		err := writeDefaultConfig(newPath)
		if err != nil {
			log.Warnf("Could not write default config file: %v", err)
			log.Warnf("Proceeding with default values")
		}
	} else {
		log.Fatalf("Could not read config file: %v", err)
	}

	viper.SetEnvPrefix("virter")
	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func isConfigNotFoundError(err error) bool {
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}

	if pathErr, ok := err.(*os.PathError); ok && pathErr.Op == "open" {
		return true
	}

	return false
}

// writeDefaultConfig renders the default config file template and writes it to path.
// The template is taken from the constant defaultConfigTemplate. The viper.Get
// function is used to substitute the actual default values in the template.
// If the operation fails in some way, the function returns with an error.
// However, this does not touch the viper configuration at all, so the default
// (or overriden) values can still be used.
func writeDefaultConfig(path string) error {
	f := template.FuncMap{
		"get": viper.Get,
	}
	tmpl := template.Must(template.New("config").Funcs(f).Parse(defaultConfigTemplate))

	defaultConfig := &bytes.Buffer{}
	err := tmpl.Execute(defaultConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	err = ioutil.WriteFile(path, defaultConfig.Bytes(), 0600)
	if err != nil {
		return fmt.Errorf("failed to write default config file: %w", err)
	}
	return nil
}

func configPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal(err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "virter")
}

func initSSHFromConfig() {
	publicPath := viper.GetString("auth.virter_public_key_path")
	if publicPath == "" {
		log.Fatal("missing configuration key: auth.virter_public_key_path")
	}

	privatePath := viper.GetString("auth.virter_private_key_path")
	if privatePath == "" {
		log.Fatal("missing configuration key: auth.virter_private_key_path")
	}

	_, err := sshkeys.NewKeyStore(privatePath, publicPath)
	if err != nil {
		log.Fatal(err)
	}
}

func getReadyConfig() virter.VmReadyConfig {
	// The config keys are kept for compatibility.
	// Note that viper.RegisterAlias sounds like it could be used, but I couldn't make it work.
	return virter.VmReadyConfig{
		Retries:      viper.GetInt("time.ssh_ping_count"),
		CheckTimeout: viper.GetDuration("time.ssh_ping_period"),
	}
}

func getDefaultContainerPullPolicy() pullpolicy.PullPolicy {
	opt := viper.GetString("container.pull")

	if opt == "" {
		return pullpolicy.IfNotExist
	}

	var policy pullpolicy.PullPolicy
	err := policy.UnmarshalText([]byte(opt))
	if err != nil {
		log.WithError(err).Fatal("invalid default pull policy")
	}

	return policy
}
