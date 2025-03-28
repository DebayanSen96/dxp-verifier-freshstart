package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dexponent/dxp-verifier/pkg/config"
	"github.com/dexponent/dxp-verifier/pkg/p2p"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./dxp-verifier start")
		os.Exit(1)
	}

	// Load default configuration
	cfg := config.DefaultConfig()

	cmd := os.Args[1]
	switch cmd {
	case "start":
		fmt.Println("Starting Dexponent verifier...")
		host, err := p2p.NewHost()
		if err != nil {
			fmt.Printf("Failed to create P2P host: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Peer ID: %s\n", host.ID().String())
		fmt.Println("Listening addresses:")
		for _, addr := range host.Addrs() {
			fmt.Printf("  %s/p2p/%s\n", addr, host.ID().String())
		}

		// Start the DHT for peer discovery
		dht, err := p2p.NewDHT(host)
		if err != nil {
			fmt.Printf("Failed to create DHT: %v\n", err)
			os.Exit(1)
		}

		// Bootstrap the DHT
		err = dht.Bootstrap()
		if err != nil {
			fmt.Printf("Failed to bootstrap DHT: %v\n", err)
			os.Exit(1)
		}

		// Start mDNS discovery if enabled
		if cfg.EnableMDNS {
			mdns, err := p2p.NewMDNS(host)
			if err != nil {
				fmt.Printf("Failed to start mDNS discovery: %v\n", err)
			} else {
				fmt.Println("mDNS discovery started")
				_ = mdns // Use the variable to avoid unused variable warning
			}
		}

		// Wait for interrupt signal
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("Shutting down...")

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Usage: ./dxp-verifier start")
		os.Exit(1)
	}
}
