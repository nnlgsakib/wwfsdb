package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".wwfsdb" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".wwfsdb")
		viper.AddConfigPath(".")
	}

	// Set default values
	viper.SetDefault("rpc-server", "http://localhost:8080/rpc")
	viper.SetDefault("ipfs-api", "localhost:5001")
	viper.SetDefault("port", "8080")
	viper.SetDefault("cache-path", "wwfsdb_cache")

	viper.SetEnvPrefix("WWSFDB")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	bindFlags()
}

func bindFlags() {
	if err := viper.BindPFlag("rpc-server", rootCmd.PersistentFlags().Lookup("rpc-server")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding rpc-server flag: %s", err)
	}
	if err := viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding config flag: %s", err)
	}
	if err := viper.BindPFlag("private-key", rootCmd.PersistentFlags().Lookup("private-key")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding private-key flag: %s", err)
	}

	// Bind serve command flags if the command exists
	if serveCmd != nil {
		if portFlag := serveCmd.Flags().Lookup("port"); portFlag != nil {
			if err := viper.BindPFlag("port", portFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error binding port flag: %s", err)
			}
		}
		if apiFlag := serveCmd.Flags().Lookup("api"); apiFlag != nil {
			if err := viper.BindPFlag("api", apiFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error binding api flag: %s", err)
			}
		}
	}
}
