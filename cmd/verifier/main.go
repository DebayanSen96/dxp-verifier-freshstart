package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dexponent/dxp-verifier/pkg/config"
	"github.com/dexponent/dxp-verifier/pkg/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./dxp-verifier [start|send]")
		fmt.Println("  start - Start the Dexponent verifier")
		fmt.Println("  send <key> <value> - Send data to all connected Dexponent peers")
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

		// Initialize the Dexponent protocol
		fmt.Println("Initializing Dexponent protocol...")
		protocol := p2p.NewDexponentProtocol(host)

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

		// Start a goroutine to attempt handshakes with new peers
		go attemptHandshakes(host, protocol)

		// Start a goroutine to periodically display Dexponent peers
		go displayDexponentPeers(protocol)
		
		// Start a goroutine to periodically check for consensus opportunities
		go runConsensusProcess(protocol)

		// Wait for interrupt signal
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("Shutting down...")

	case "send":
		// Check for required arguments
		if len(os.Args) < 4 {
			fmt.Println("Usage: ./dxp-verifier send <key> <value>")
			os.Exit(1)
		}

		key := os.Args[2]
		value := os.Args[3]

		// Create a temporary host for sending data
		host, err := p2p.NewHost()
		if err != nil {
			fmt.Printf("Failed to create P2P host: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Temporary peer ID: %s\n", host.ID().String())

		// Initialize the Dexponent protocol
		protocol := p2p.NewDexponentProtocol(host)

		// Start mDNS discovery to find local peers
		mdns, err := p2p.NewMDNS(host)
		if err != nil {
			fmt.Printf("Failed to start mDNS discovery: %v\n", err)
			os.Exit(1)
		}
		_ = mdns // Use the variable to avoid unused variable warning

		fmt.Println("Searching for Dexponent peers...")

		// Wait a bit for peer discovery
		time.Sleep(5 * time.Second)

		// Broadcast the data to all connected Dexponent peers
		fmt.Printf("Broadcasting data - Key: %s, Value: %s\n", key, value)
		err = protocol.BroadcastData(key, value)
		if err != nil {
			fmt.Printf("Error broadcasting data: %v\n", err)
		}

		// Wait a bit for the message to be sent
		time.Sleep(2 * time.Second)
		fmt.Println("Done.")

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Usage: ./dxp-verifier [start|send]")
		os.Exit(1)
	}
}

// attemptHandshakes periodically checks for new peers and attempts to handshake with them
func attemptHandshakes(host p2p.Host, protocol *p2p.DexponentProtocol) {
	knownPeers := make(map[peer.ID]bool)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Get all connected peers
			peers := host.Network().Peers()

			// Attempt handshake with new peers
			for _, peerID := range peers {
				// Skip if we already know this peer
				if knownPeers[peerID] {
					continue
				}

				// Skip if this is already a Dexponent peer
				if protocol.IsDexponentPeer(peerID) {
					knownPeers[peerID] = true
					continue
				}

				// Attempt to handshake with this peer - don't log the attempt or errors
				// Only log successful handshakes (which happens in the protocol)
				_ = protocol.SendHandshake(peerID)
				// Silently ignore all errors when connecting to public peers
				// Mark as known regardless of outcome to avoid repeated attempts
				knownPeers[peerID] = true
			}
		}
	}
}

// displayDexponentPeers periodically displays the list of Dexponent peers
func displayDexponentPeers(protocol *p2p.DexponentProtocol) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			peers := protocol.GetDexponentPeers()
			fmt.Printf("Connected to %d Dexponent peers:\n", len(peers))
			for _, peerID := range peers {
				fmt.Printf("  Dexponent Peer: %s\n", peerID.String())
			}
		}
	}
}

// runConsensusProcess periodically checks if we can start a consensus round
func runConsensusProcess(protocol *p2p.DexponentProtocol) {
	// Wait for initial peer discovery
	time.Sleep(10 * time.Second)
	
	// Check for consensus opportunities every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check if we have enough peers for consensus (at least 3)
			peers := protocol.GetDexponentPeers()
			if len(peers) >= 2 { // At least 2 other peers (3 total including us)
				// Try to start consensus process
				protocol.StartConsensusProcess()
			}
		}
	}
}
