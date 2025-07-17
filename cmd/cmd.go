package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/decentrio/gateway/config"
	"github.com/decentrio/gateway/gateway"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
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
		// fmt.Printf("%+v\n", cfg)
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

var testMultiRequestCmd = &cobra.Command{
	Use:   "test-multi-request",
	Short: "test multi-request",
	Run: func(cmd *cobra.Command, args []string) {
		conn, err := grpc.Dial("localhost:5002", grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		client := tmservice.NewServiceClient(conn)

		var wg sync.WaitGroup
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				// Giả sử block height là 123
				res, err := client.GetBlockByHeight(ctx, &tmservice.GetBlockByHeightRequest{
					Height: 123,
				})
				if err != nil {
					fmt.Printf("Request %d error: %v\n", i, err)
					return
				}
				fmt.Printf("Request %d block ID: %s\n", i, res.BlockId.String())
			}(i)
		}
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(testMultiRequestCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	startCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
