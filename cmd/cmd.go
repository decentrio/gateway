package cmd

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
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
	Args:  cobra.ExactArgs(2),
	Use:   "test-multi-request-grpc <num-req> <req-par>",
	Short: "Send num-req gRPC requests with max req-par goroutines in parallel",
	Run: func(cmd *cobra.Command, args []string) {
		numReq, err := strconv.Atoi(args[0])
		if err != nil || numReq < 1 {
			fmt.Printf("request: %s\n", args[0])
			os.Exit(1)
		}
		reqPar, err := strconv.Atoi(args[1])
		if err != nil || reqPar < 1 {
			fmt.Printf("goroutine: %s\n", args[1])
			os.Exit(1)
		}

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

		sem := make(chan struct{}, reqPar)
		var wg sync.WaitGroup

		for i := 0; i < numReq; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
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
				return
			}(i)
		}
		wg.Wait()
		fmt.Printf("Total successful requests: %d\n", suc)
		fmt.Printf("Total execution time: %s\n", time.Since(start))
	},
}

var testMultiRequestRPCCmd = &cobra.Command{
	Args:  cobra.ExactArgs(2),
	Use:   "test-multi-request-rpc <num-req> <req-par>",
	Short: "Send num-req RPC requests with max req-par goroutines in parallel",
	Run: func(cmd *cobra.Command, args []string) {
		numReq, err := strconv.Atoi(args[0])
		if err != nil || numReq < 1 {
			fmt.Printf("request: %s\n", args[0])
			os.Exit(1)
		}
		reqPar, err := strconv.Atoi(args[1])
		if err != nil || reqPar < 1 {
			fmt.Printf("goroutine: %s\n", args[1])
			os.Exit(1)
		}

		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			os.Exit(1)
		}
		blocks := cfg.Upstream[len(cfg.Upstream)-1].Blocks
		heightMax := blocks[len(blocks)-1]

		const endpoint = "http://localhost:5001"

		sem := make(chan struct{}, reqPar)
		var wg sync.WaitGroup
		var suc int32
		start := time.Now()
		for i := 0; i < numReq; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()

				randHeight := int64(rand.Uint64() % (heightMax + 1))
				client := http.Client{Timeout: 5 * time.Second}
				url := fmt.Sprintf("%s/block?height=%d", endpoint, randHeight)

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
				return
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
