/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>
This file is part of the Advocat AI platform.
*/
package cmd

import (
	"fmt"
	"github.com/advocat-ai/pandoc-converter/api"
	"github.com/advocat-ai/pandoc-converter/internal"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

const (
	kVerboseFlag = "verbose"
	kVerboseFlagP = "v"
	kVerboseDefault = false
	kListenProtocolFlag = "protocol"
	kListenProtocolFlagP = "t"
	kListenProtocolDefault = "tcp"
	kBindAddressFlag = "bind"
	kBindAddressFlagP = "b"
	kBindAddressDefault = "0.0.0.0:5000"
	kPandocPathFlag = "pandoc-path"
	kPandocPathP = "x"
	kPandocPathDefault = "/usr/local/bin/pandoc"
	kPathEnvFlag = "path-env-var"
	kPathEnvFlagP = "p"
	kPathEnvDefault = "/opt/texlive/texdir/bin/x86_64-linuxmusl:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	kTlsKeyPathFlag = "tls-key"
	kTlsKeyPathFlagP = "k"
	kTlsKeyPathDefault = ""
	kTlsCrtPathFlag = "tls-crt"
	kTlsCrtPathFlagP = "c"
	kTlsCrtPathDefault = ""
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pandoc-converter",
	Short: "Exposes the pandoc utility as a gRPC service",
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose := viper.GetBool(kVerboseFlag)

		var err error

		var log *zap.Logger

		if verbose {
			log, err = zap.NewDevelopment()
			if err != nil {
				return err
			}
		} else {
			log, err = zap.NewProduction()
			if err != nil {
				return err
			}
		}

		protocol := viper.GetString(kListenProtocolFlag)
		address := viper.GetString(kBindAddressFlag)
		log = log.With(zap.String("bindAddress", fmt.Sprintf("%v://%v", protocol, address)))
		log.Debug("creating listener")
		listener, err := net.Listen(protocol, address)
		if err != nil {
			log.Error("failed to create listener", zap.Error(err))
			return err
		}
		log.Debug("listener created")

		var server *grpc.Server

		tlsKeyPath := viper.GetString(kTlsKeyPathFlag)
		tlsCrtPath := viper.GetString(kTlsCrtPathFlag)
		if tlsKeyPath != "" && tlsCrtPath != "" {
			log = log.With(zap.String("tlsKey", tlsKeyPath), zap.String("tlsCrt", tlsCrtPath))
			log.Debug("creating credentials")
			creds, err := credentials.NewServerTLSFromFile(tlsCrtPath, tlsKeyPath)
			if err != nil {
				log.Error("failed to create credentials", zap.Error(err))
				return err
			}
			log.Debug("creating server with secure transport")
			server = grpc.NewServer(grpc.Creds(creds))
		} else {
			log.Debug("creating server with insecure transport")
			server = grpc.NewServer()
		}

		pandocPath := viper.GetString(kPandocPathFlag)
		pathEnv := viper.GetString(kPathEnvFlag)

		log.Debug("creating service")
		service, err := internal.NewConverterService(internal.WithLog(log), internal.WithPandocPath(pandocPath), internal.WithPathEnvVar(pathEnv))

		if err != nil {
			log.Error("failed to create service", zap.Error(err))
			return err
		}

		log.Debug("registering service with server")
		api.RegisterConverterServer(server, service)

		log.Debug("starting gRPC server")
		err = server.Serve(listener)
		if err != nil {
			log.Error("failed to start gRPC server", zap.Error(err))
			return err
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	viper.SetDefault(kVerboseFlag, kVerboseDefault)
	viper.SetDefault(kListenProtocolFlag, kListenProtocolDefault)
	viper.SetDefault(kBindAddressFlag, kBindAddressDefault)
	viper.SetDefault(kPandocPathFlag, kPandocPathDefault)
	viper.SetDefault(kPathEnvFlag, kPathEnvDefault)
	viper.SetDefault(kTlsKeyPathFlag, kTlsKeyPathDefault)
	viper.SetDefault(kTlsCrtPathFlag, kTlsCrtPathDefault)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pandoc-converter.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP(kVerboseFlag, kVerboseFlagP, kVerboseDefault, "Enabled verbose logging")
	rootCmd.Flags().StringP(kListenProtocolFlag, kListenProtocolFlagP, kListenProtocolDefault, "Protocol to use for the server binding")
	rootCmd.Flags().StringP(kBindAddressFlag, kBindAddressFlagP, kBindAddressDefault, "Address or path to bind to for the server")
	rootCmd.Flags().StringP(kPandocPathFlag, kPandocPathP, kPandocPathDefault, "Path to the pandoc executable")
	rootCmd.Flags().StringP(kPathEnvFlag, kPathEnvFlagP, kPathEnvDefault, "The PATH environment variable for the process")
	rootCmd.Flags().StringP(kTlsKeyPathFlag, kTlsKeyPathFlagP, kTlsKeyPathDefault, "For encrypted transport, the path to the TLS private key file")
	rootCmd.Flags().StringP(kTlsCrtPathFlag, kTlsCrtPathFlagP, kTlsCrtPathDefault, "For encrypted transport, the path to the TLS certificate file")
	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		panic(err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".pandoc-converter" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".pandoc-converter")
	}

	viper.SetEnvPrefix("PANDOC_CONVERTER_")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
