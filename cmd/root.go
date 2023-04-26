package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"

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
var logFormat string

type ShortFormatter struct {
	LevelDesc []string
}

func (f *ShortFormatter) Format(entry *log.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("%s %s\n", f.LevelDesc[entry.Level], entry.Message)), nil
}

func rootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "virter",
		Version: version,
		Short:   "Virter manages local virtual machines",
		Long: `Virter manages local virtual machines for development and testing. The
machines are controlled with libvirt, with qcow2 chained images for storage
and cloud-init for basic access configuration. This allows for fast cloning
and resetting, for a stable test environment.`,
	}

	configName := filepath.Join(configPath(), "virter.toml")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is %v)", configName))
	rootCmd.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", defaultLogLevel, "Log level")
	rootCmd.PersistentFlags().StringVar(&logFormat, "logformat", "default", "Log format, current options: short")

	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(imageCommand())
	rootCmd.AddCommand(vmCommand())
	rootCmd.AddCommand(networkCommand())
	rootCmd.AddCommand(registryCommand())
	return rootCmd
}

func initLogging() {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(level)
	if logFormat == "short" {
		shortFormatter := new(ShortFormatter)
		shortFormatter.LevelDesc = []string{"PANC", "FATL", "ERRO", "WARN", "INFO", "DEBG"}
		log.SetFormatter(shortFormatter)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cobra.OnInitialize(initLogging, initConfig, initSSHFromConfig)

	if err := rootCommand().ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}

func suggestNone(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
