package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

func rootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "virter",
		Short: "Virter manages local virtual machines",
		Long: `Virter manages local virtual machines for development and testing. The
machines are controlled with libvirt, with qcow2 chained images for storage
and cloud-init for basic access configuration. This allows for fast cloning
and resetting, for a stable test environment.`,
	}

	configName := filepath.Join(configPath(), "virter.toml")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is %v)", configName))

	rootCmd.AddCommand(imageCommand())
	rootCmd.AddCommand(vmCommand())
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	if err := rootCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetDefault("libvirt.pool", "default")
	viper.SetDefault("libvirt.network", "default")
	viper.SetDefault("libvirt.template_dir", "assets/libvirt-templates")
	viper.SetDefault("image.registry", "assets/images.toml")
	viper.SetDefault("ping.count", 60)
	viper.SetDefault("ping.period", time.Second)

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
