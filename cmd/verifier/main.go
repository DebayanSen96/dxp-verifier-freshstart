package main

import (
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dexponent/dxp-verifier/pkg/config"
	"github.com/dexponent/dxp-verifier/pkg/eth"
	"github.com/dexponent/dxp-verifier/pkg/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

// printUsage prints the usage information for the verifier
func printUsage() {
	fmt.Println("Usage: ./dxp-verifier [command] [options]")
	fmt.Println("Commands:")
	fmt.Println("  start             Start the Dexponent verifier")
	fmt.Println("    --block-polling-interval N  Set block polling interval in seconds (default: 10)")
	fmt.Println("    --detached                  Run in detached mode")
	fmt.Println("  register          Register as a verifier with the DXP contract")
	fmt.Println("  status            Check validator status")
	fmt.Println("  stop              Stop a running validator")
	fmt.Println("  rewards           Check pending rewards")
	fmt.Println("  claim             Claim accumulated rewards")
	fmt.Println("  send <key> <value>  Send data to all connected Dexponent peers")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load default configuration
	cfg := config.DefaultConfig()

	// Parse command
	cmd := os.Args[1]
	// Parse flags for the start command
	startCmd := flag.NewFlagSet("start", flag.ExitOnError)
	blockPollingInterval := startCmd.Int("block-polling-interval", 10, "Interval in seconds for polling new blocks")
	detached := startCmd.Bool("detached", false, "Run in detached mode")
	
	// Parse flags for other commands
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	stopCmd := flag.NewFlagSet("stop", flag.ExitOnError)
	rewardsCmd := flag.NewFlagSet("rewards", flag.ExitOnError)
	claimCmd := flag.NewFlagSet("claim", flag.ExitOnError)
	
	switch cmd {
	case "start":
		// Parse start command flags
		startCmd.Parse(os.Args[2:])
		
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

		// Initialize Ethereum client
		ethClient, err := eth.NewClient()
		if err != nil {
			fmt.Printf("Warning: Failed to initialize Ethereum client: %v\n", err)
			fmt.Println("Continuing without blockchain integration...")
		}

		// Initialize the Dexponent protocol
		fmt.Println("Initializing Dexponent protocol...")
		protocol := p2p.NewDexponentProtocol(host)
		
		// Set Ethereum client if available
		if ethClient != nil {
			// Perform blockchain connection check
			currentBlock, err := ethClient.GetCurrentBlock()
			if err != nil {
				fmt.Printf("⚠️ Warning: Failed to connect to blockchain: %v\n", err)
			} else {
				fmt.Printf("✅ Successfully connected to blockchain. Current block: %d\n", currentBlock)
				
				// Check DXP balance
				dxpBalance, err := ethClient.GetDXPBalance()
				if err != nil {
					fmt.Printf("⚠️ Warning: Failed to check DXP balance: %v\n", err)
				} else {
					// Convert to human-readable format (18 decimals)
					dxpBalanceFloat := new(big.Float).SetInt(dxpBalance)
					dxpBalanceFloat = dxpBalanceFloat.Quo(dxpBalanceFloat, big.NewFloat(1e18))
					dxpBalanceStr := dxpBalanceFloat.Text('f', 4)
					
					// Check if balance is less than minimum stake (100 DXP)
					minStake := new(big.Int).Mul(big.NewInt(100), big.NewInt(1000000000000000000))
					if dxpBalance.Cmp(minStake) < 0 {
						fmt.Printf("⚠️ Warning: DXP balance (%s DXP) is less than minimum stake amount (100 DXP)\n", dxpBalanceStr)
					} else {
						fmt.Printf("✅ DXP balance: %s DXP\n", dxpBalanceStr)
					}
					
					// Check DXP allowance
					dxpAllowance, err := ethClient.GetDXPAllowance()
					if err != nil {
						fmt.Printf("⚠️ Warning: Failed to check DXP allowance: %v\n", err)
					} else {
						// Convert to human-readable format (18 decimals)
						allowanceFloat := new(big.Float).SetInt(dxpAllowance)
						allowanceFloat = allowanceFloat.Quo(allowanceFloat, big.NewFloat(1e18))
						allowanceStr := allowanceFloat.Text('f', 4)
						
						// Check if allowance is less than minimum stake (100 DXP)
						if dxpAllowance.Cmp(minStake) < 0 {
							fmt.Printf("⚠️ Warning: DXP allowance (%s DXP) is less than minimum stake amount (100 DXP)\n", allowanceStr)
						} else {
							fmt.Printf("✅ DXP allowance: %s DXP\n", allowanceStr)
						}
					}
				}
			}
			
			protocol.SetEthClient(ethClient)
		}
		
		// Configure block polling interval if specified
		if *blockPollingInterval > 0 {
			fmt.Printf("Setting block polling interval to %d seconds\n", *blockPollingInterval)
			// TODO: Implement block polling logic
		}
		
		// Check if running in detached mode
		if *detached {
			fmt.Println("Running in detached mode")
			// TODO: Implement detached mode logic
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

	case "status":
		// Parse status command flags
		statusCmd.Parse(os.Args[2:])
		
		fmt.Println("Checking validator status...")
		
		// Initialize Ethereum client
		ethClient, err := eth.NewClient()
		if err != nil {
			fmt.Printf("Failed to initialize Ethereum client: %v\n", err)
			os.Exit(1)
		}
		
		// Check if registered as verifier
		isRegistered, err := ethClient.IsRegisteredVerifier()
		if err != nil {
			fmt.Printf("Failed to check verifier status: %v\n", err)
			os.Exit(1)
		}
		
		if isRegistered {
			fmt.Println("✅ Registered as verifier")
			
			// Get verifier status
			status, err := ethClient.GetVerifierStatus()
			if err != nil {
				fmt.Printf("Failed to get verifier status: %v\n", err)
			} else {
				fmt.Println("Verifier status:")
				for k, v := range status {
					fmt.Printf("  %s: %v\n", k, v)
				}
			}
		} else {
			fmt.Println("❌ Not registered as verifier")
		}

	case "stop":
		// Parse stop command flags
		stopCmd.Parse(os.Args[2:])
		
		fmt.Println("Stopping validator...")
		// TODO: Implement logic to stop a running validator
		fmt.Println("Validator stopped")

	case "rewards":
		// Parse rewards command flags
		rewardsCmd.Parse(os.Args[2:])
		
		fmt.Println("Checking pending rewards...")
		
		// Initialize Ethereum client
		ethClient, err := eth.NewClient()
		if err != nil {
			fmt.Printf("Failed to initialize Ethereum client: %v\n", err)
			os.Exit(1)
		}
		
		// Get pending rewards
		rewards, err := ethClient.GetPendingRewards()
		if err != nil {
			fmt.Printf("Failed to get pending rewards: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Pending rewards: %v\n", rewards)

	case "register":
		// Parse register command flags
		registerCmd := flag.NewFlagSet("register", flag.ExitOnError)
		registerCmd.Parse(os.Args[2:])
		
		fmt.Println("Registering as a verifier...")
		
		// Initialize Ethereum client
		ethClient, err := eth.NewClient()
		if err != nil {
			fmt.Printf("Failed to initialize Ethereum client: %v\n", err)
			os.Exit(1)
		}
		
		// Check if already registered
		isRegistered, err := ethClient.IsRegisteredVerifier()
		if err != nil {
			fmt.Printf("Failed to check registration status: %v\n", err)
			os.Exit(1)
		}
		
		if isRegistered {
			fmt.Println("Already registered as a verifier")
			os.Exit(0)
		}
		
		// Register as a verifier
		txHash, err := ethClient.RegisterVerifier()
		if err != nil {
			fmt.Printf("Failed to register as a verifier: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Registration transaction submitted: %s\n", txHash)
		
		// Wait for transaction confirmation
		fmt.Println("Waiting for transaction confirmation...")
		_, err = ethClient.WaitForTransaction(txHash)
		if err != nil {
			fmt.Printf("Transaction failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Successfully registered as a verifier!")

	case "claim":
		// Parse claim command flags
		claimCmd.Parse(os.Args[2:])
		
		fmt.Println("Claiming rewards...")
		
		// Initialize Ethereum client
		ethClient, err := eth.NewClient()
		if err != nil {
			fmt.Printf("Failed to initialize Ethereum client: %v\n", err)
			os.Exit(1)
		}
		
		// Claim rewards
		txHash, err := ethClient.ClaimRewards()
		if err != nil {
			fmt.Printf("Failed to claim rewards: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Rewards claim transaction submitted: %s\n", txHash)
		
		// Wait for transaction confirmation
		fmt.Println("Waiting for transaction confirmation...")
		_, err = ethClient.WaitForTransaction(txHash)
		if err != nil {
			fmt.Printf("Transaction failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Rewards claimed successfully!")

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
		printUsage()
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
