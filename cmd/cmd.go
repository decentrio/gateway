package cmd

import (
	"fmt"
	"os"
	
	"github.com/decentrio/gateway/config"
	"github.com/decentrio/gateway/gateway"
	"github.com/spf13/cobra"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Gateway CLI",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		config.GenerateConfig()
		fmt.Println("Configuration file created successfully.")
		return
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the gateway",
	PreRun: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%+v\n", cfg)
		config.SetConfig(cfg)
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Starting gateway with config file: %s\n", configFile)
		gw, err := gateway.NewGateway(config.GetConfig())
		if err != nil {
			fmt.Printf("Error creating gateway: %v\n", err)
			os.Exit(1)
		}
		
		gw.Start()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	startCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}