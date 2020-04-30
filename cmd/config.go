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

# template_dir is the directory where virters libvirt template files are stored.
# These are static and normally do not have to be touched by the user.
# Relative paths will be interpreted as relative to the current working directory
# when executing virter.
# Default value: "{{ get "libvirt.template_dir" }}"
template_dir = "{{ get "libvirt.template_dir" }}"

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

# docker_timeout is how long virter will wait for a Docker provisioning step to
# complete.
# If a Docker provisioning step exceeds this timeout, it will be aborted and an
# error will be produced.
# Default value: "{{ get "time.docker_timeout" }}"
docker_timeout = "{{ get "time.docker_timeout" }}"

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
`

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetDefault("libvirt.pool", "default")
	viper.SetDefault("libvirt.network", "default")
	viper.SetDefault("libvirt.template_dir", "assets/libvirt-templates")
	viper.SetDefault("time.ssh_ping_count", 60)
	viper.SetDefault("time.ssh_ping_period", time.Second)
	viper.SetDefault("time.shutdown_timeout", 20*time.Second)
	viper.SetDefault("time.docker_timeout", 30*time.Minute)

	viper.SetConfigType("toml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		p := configPath()
		os.MkdirAll(p, 0700)

		// Use config file from standard location
		viper.AddConfigPath(p)
		viper.SetConfigName("virter")

		// When using the default config file location, make that also the default key location
		viper.SetDefault("auth.virter_public_key_path", filepath.Join(p, "id_rsa.pub"))
		viper.SetDefault("auth.virter_private_key_path", filepath.Join(p, "id_rsa"))
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Print("Using config file: ", viper.ConfigFileUsed())
	} else if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		newPath := filepath.Join(configPath(), "virter.toml")
		log.Print("Config file does not exist, creating default: ", newPath)

		err := writeDefaultConfig(newPath)
		if err != nil {
			log.Warnf("Could not write default config file: %v", err)
			log.Warnf("Proceeding with default values")
		}
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

	err = ioutil.WriteFile(path, defaultConfig.Bytes(), 0700)
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

	err := initSSHKeys(publicPath, privatePath)
	if err != nil {
		log.Fatal(err)
	}
}
