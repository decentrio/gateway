package cmd

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
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

var testMultiRequestGRPCCmd = &cobra.Command{
	Use:   "test-multi-request-grpc",
	Short: "test-multi-request-grpc will send 8000 requests (with different heights) to nodes, 100 requests each time in parallel",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			os.Exit(1)
		}

		blocks := cfg.Upstream[len(cfg.Upstream)-1].Blocks
		heightMax := blocks[len(blocks)-1]

		conn, err := grpc.Dial("localhost:5002", grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		client := tmservice.NewServiceClient(conn)

		var suc int32
		start := time.Now()
		const maxConcurrency = 100

		sem := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup

		for i := 0; i < 8000; i++ {
			wg.Add(1)
			sem <- struct{}{} // chiếm 1 slot

			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }() // giải phóng slot khi xong

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				randHeight := int64(rand.Uint64() % (heightMax + 1))
				res, err := client.GetBlockByHeight(ctx, &tmservice.GetBlockByHeightRequest{
					Height: randHeight,
				})
				if err != nil {
					fmt.Printf("Request %d error: %v\n", i, err)
					return
				}
				fmt.Printf("Request %d block ID: %s\n", i, res.BlockId.String())
				atomic.AddInt32(&suc, 1)
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)
		fmt.Printf("Total successful requests: %d\n", suc)
		fmt.Printf("Total execution time: %s\n", duration)
	},
}

var testMultiRequestRPCCmd = &cobra.Command{
	Use:   "test-multi-request-rpc",
	Short: "Test 30 concurrent JSON-RPC HTTP requests",
	Run: func(cmd *cobra.Command, args []string) {
		const endpoint = "http://localhost:5001" // Tendermint RPC mặc định

		// const maxConcurrentRequests = 10
		// semaphore := make(chan struct{}, maxConcurrentRequests)

		var wg sync.WaitGroup
		var suc int32
		start := time.Now()
		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				// semaphore <- struct{}{}
				// defer func() { <-semaphore }()

				client := http.Client{Timeout: 60 * time.Second}
				url := fmt.Sprintf("%s/block?height=%d", endpoint, 123)

				resp, err := client.Get(url)
				if err != nil {
					fmt.Printf("[RPC] Request %d error: %v\n", i, err)
					return
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					fmt.Printf("[RPC] Request %d read error: %v\n\n", i, err)
					return
				}

				fmt.Printf("[RPC] Request %d response: %s\n\n", i, string(body))
				atomic.AddInt32(&suc, 1)
			}(i)
		}
		wg.Wait()
		duration := time.Since(start)
		fmt.Printf("Total successful requests: %d\n", suc)
		fmt.Printf("Total execution time: %s\n", duration)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(testMultiRequestGRPCCmd)
	rootCmd.AddCommand(testMultiRequestRPCCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	startCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
