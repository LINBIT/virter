package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "virter",
	Short: "Virter manages local virtual machines",
	Long: `Virter manages local virtual machines for development and testing. The
machines are controlled with libvirt, with qcow2 chained images for storage
and cloud-init for basic access configuration. This allows for fast cloning
and resetting, for a stable test environment.`,
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	configName := filepath.Join(configPath(), "virter.toml")
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is %v)", configName))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetDefault("libvirt.pool", "default")
	viper.SetDefault("libvirt.template_dir", "assets/libvirt-templates")

	viper.SetConfigType("toml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Use config file from standard location
		viper.AddConfigPath(configPath())
		viper.SetConfigName("virter")
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
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
