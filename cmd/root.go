package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var defaultLogLevel = log.InfoLevel.String()

var cfgFile string
var logLevel string

func rootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "virter",
		Short: "Virter manages local virtual machines",
		Long: `Virter manages local virtual machines for development and testing. The
machines are controlled with libvirt, with qcow2 chained images for storage
and cloud-init for basic access configuration. This allows for fast cloning
and resetting, for a stable test environment.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level, err := log.ParseLevel(logLevel)
			if err != nil {
				log.Fatal(err)
			}
			log.SetLevel(level)
		},
	}

	configName := filepath.Join(configPath(), "virter.toml")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is %v)", configName))
	rootCmd.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", defaultLogLevel, "Log level")

	rootCmd.AddCommand(imageCommand())
	rootCmd.AddCommand(vmCommand())
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	cobra.OnInitialize(initConfig, initSSHFromConfig)

	if err := rootCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetDefault("libvirt.pool", "default")
	viper.SetDefault("libvirt.network", "default")
	viper.SetDefault("libvirt.template_dir", "assets/libvirt-templates")
	viper.SetDefault("image.registry", "assets/images.toml")
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

		// Use config file from standard location
		viper.AddConfigPath(p)
		viper.SetConfigName("virter")

		// When using the default config file location, make that also the default key location
		viper.SetDefault("auth.virter_public_key_path", filepath.Join(p, "id_rsa.pub"))
		viper.SetDefault("auth.virter_private_key_path", filepath.Join(p, "id_rsa"))
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Print("Using config file: ", viper.ConfigFileUsed())
	}
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
