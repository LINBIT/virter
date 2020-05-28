package cmd

import (
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

// potentially injected by makefile
var (
	version   string
	builddate string
	githash   string
)

var defaultLogLevel = log.InfoLevel.String()

var cfgFile string
var logLevel string

func rootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "virter",
		Version: version,
		Short:   "Virter manages local virtual machines",
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

	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(imageCommand())
	rootCmd.AddCommand(vmCommand())
	rootCmd.AddCommand(registryCommand())
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	cobra.OnInitialize(initConfig, initSSHFromConfig)

	if err := rootCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
