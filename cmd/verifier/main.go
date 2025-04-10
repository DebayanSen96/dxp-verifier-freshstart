package main

import (
	"flag"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/dexponent/dxp-verifier/pkg/eth"
	"github.com/dexponent/dxp-verifier/pkg/logger"
	"github.com/dexponent/dxp-verifier/pkg/p2p"
)

// printUsage prints the usage information for the verifier
func printUsage() {
	fmt.Println("Usage: ./dxp-verifier [command] [options]")
	fmt.Println("Commands:")
	fmt.Println("  start             Start the Dexponent verifier")
	fmt.Println("    --block-polling-interval N  Set block polling interval in seconds (default: 10)")
	fmt.Println("    --detached                  Run in detached mode")
	fmt.Println("  register          Register as a verifier with the DXP contract")
	fmt.Println("    --amount N        Amount of DXP tokens to stake (required)")
	fmt.Println("    --farmid N        Farm ID to register for (1-8, required)")
	fmt.Println("  status            Check validator status and metrics")
	fmt.Println("  stop              Stop a running validator")
	fmt.Println("  claim-rewards     Claim accumulated rewards")
	fmt.Println("  send <key> <value>  Send data to all connected Dexponent peers")
	fmt.Println("  withdraw          Withdraw verifier stake")
	fmt.Println("    --amount N        Amount of stake to withdraw")
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Warning: Error loading .env file: %v\n", err)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Parse command
	cmd := os.Args[1]
	
	// Create logs directory
	logsDir := "logs"
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		if err := os.Mkdir(logsDir, 0755); err != nil {
			fmt.Printf("Warning: Failed to create logs directory: %v\n", err)
		}
	}
	
	// Initialize logger
	if err := logger.Init(logsDir, true); err != nil {
		fmt.Printf("Warning: Failed to initialize logger: %v\n", err)
	}
	defer logger.Close()

	// Parse flags for the start command
	startCmd := flag.NewFlagSet("start", flag.ExitOnError)
	detachedMode := startCmd.Bool("detached", false, "Run in detached mode")

	// Parse flags for other commands
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	claimCmd := flag.NewFlagSet("claim-rewards", flag.ExitOnError)
	stopCmd := flag.NewFlagSet("stop", flag.ExitOnError)
	registerCmd := flag.NewFlagSet("register", flag.ExitOnError)
	registerAmount := registerCmd.Int("amount", 0, "Amount of DXP tokens to stake (required)")
	farmId := registerCmd.Int("farmid", 0, "Farm ID to register for (1-8, required)")
	withdrawCmd := flag.NewFlagSet("withdraw", flag.ExitOnError)
	withdrawAmount := withdrawCmd.Int("amount", 0, "Amount of stake to withdraw")

	// Get Ethereum configuration from environment variables
	rpcURL := os.Getenv("BASE_RPC_URL")
	privateKeyHex := os.Getenv("WALLET_PRIVATE_KEY")
	contractAddress := os.Getenv("DXP_CONTRACT_ADDRESS")
	tokenAddress := os.Getenv("DXP_TOKEN_ADDRESS")

	// Use default token address if not provided
	if tokenAddress == "" {
		tokenAddress = eth.DefaultDXPTokenAddress
	}

	switch cmd {
	case "start":
		// Parse flags
		err := startCmd.Parse(os.Args[2:])
		if err != nil {
			printUsage()
			os.Exit(1)
		}

		// Check if running in detached mode
		if *detachedMode {
			// Get path to executable
			executable, err := os.Executable()
			if err != nil {
				fmt.Printf("Failed to get executable path: %v\n", err)
				os.Exit(1)
			}
			
			// Create command with the same arguments but without detached flag
			args := []string{"start"}
			for _, arg := range os.Args[2:] {
				if arg != "--detached" && arg != "-detached" {
					args = append(args, arg)
				}
			}
			
			// Create a new process
			cmd := exec.Command(executable, args...)
			cmd.Stdout = nil
			cmd.Stderr = nil
			
			// Start the process
			err = cmd.Start()
			if err != nil {
				fmt.Printf("Failed to start detached process: %v\n", err)
				os.Exit(1)
			}
			
			// Write PID to file
			pidFile := "dxp-verifier.pid"
			err = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)
			if err != nil {
				fmt.Printf("Warning: Failed to write PID file: %v\n", err)
			}
			
			fmt.Printf("Verifier started in detached mode with PID %d\n", cmd.Process.Pid)
			os.Exit(0)
		}
		
		logger.Info("Initializing P2P host...")
		host, err := p2p.NewHost()
		if err != nil {
			logger.Error("Failed to initialize P2P host: %v", err)
			os.Exit(1)
		}
		
		// Log peer ID
		peerID := host.ID().String()
		logger.Success("Peer ID: %s", peerID)
		
		// Initialize mDNS discovery service
		_, err = p2p.NewMDNS(host)
		if err != nil {
			logger.Warn("Failed to initialize mDNS discovery: %v", err)
		} else {
			logger.Success("mDNS discovery service started")
		}

		// Initialize Ethereum client
		ethClient, err := eth.NewClient(rpcURL, privateKeyHex, contractAddress, tokenAddress)
		if err != nil {
			logger.Warn("Failed to initialize Ethereum client: %v", err)
			logger.Info("Continuing without blockchain integration...")
		}

		// Initialize the Dexponent protocol
		logger.Info("Initializing Dexponent protocol...")
		protocol := p2p.NewDexponentProtocol(host)

		// Set Ethereum client if available
		if ethClient != nil {
			// Perform blockchain connection check
			currentBlock, err := ethClient.GetCurrentBlock()
			if err != nil {
				logger.Warn("Failed to get current block: %v", err)
			} else {
				logger.Success("Connected to blockchain at block %d", currentBlock)
			}

			// Check if registered as verifier
			isRegistered, err := ethClient.IsRegisteredVerifier()
			if err != nil {
				logger.Warn("Failed to check verifier status: %v", err)
			} else if isRegistered {
				logger.Success("Registered as a verifier")
				
				// Get assigned farms
				farms, err := ethClient.GetAssignedFarms()
				if err != nil {
					logger.Warn("Failed to get assigned farms: %v", err)
				} else if len(farms) > 0 {
					farmInfo := fmt.Sprintf("Assigned to %d farms: ", len(farms))
					for i, farmID := range farms {
						if i > 0 {
							farmInfo += ", "
						}
						farmInfo += fmt.Sprintf("%d", farmID)
						
						// Check if active for this farm
						isActive, err := ethClient.IsVerifierActiveForFarm(farmID)
						if err == nil && isActive {
							farmInfo += " (Active)"
						}
					}
					logger.Success(farmInfo)
					
					// Start benchmark updater for active farms
					go runBenchmarkUpdater(ethClient, farms, 60*time.Second)
				} else {
					logger.Info("Not assigned to any farms")
				}
			} else {
				fmt.Println("Not registered as a verifier")
			}
		}

		// Start peer discovery
		logger.Info("Starting peer discovery...")
		
		// Start consensus process in background
		go runConsensusProcess(protocol)
		
		logger.Success("Verifier started successfully!")

		// Start periodic handshake attempts with new peers
		go attemptHandshakes(host, protocol)

		// Start periodic display of connected Dexponent peers
		go displayDexponentPeers(protocol)

		// Wait for interrupt signal
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("Shutting down...")

	case "register":
		// Parse register command flags
		registerCmd.Parse(os.Args[2:])

		// Check if required flags are provided
		if *registerAmount <= 0 {
			logger.Error("Error: --amount flag is required and must be greater than 0")
			os.Exit(1)
		}

		if *farmId <= 0 || *farmId > 8 {
			logger.Error("Error: --farmid flag is required and must be between 1 and 8")
			os.Exit(1)
		}

		// Initialize Ethereum client
		ethClient, err := eth.NewClient(rpcURL, privateKeyHex, contractAddress, tokenAddress)
		if err != nil {
			logger.Error("Failed to initialize Ethereum client: %v", err)
			os.Exit(1)
		}

		// Check if already registered
		isRegistered, err := ethClient.IsRegisteredVerifier()
		if err != nil {
			logger.Error("Failed to check verifier status: %v", err)
			os.Exit(1)
		}

		if isRegistered {
			logger.Info("Already registered as a verifier")
			os.Exit(0)
		}

		// Convert amount to wei
		amountWei, err := ethClient.ConvertToWei(fmt.Sprintf("%d", *registerAmount))
		if err != nil {
			logger.Error("Failed to convert amount to wei: %v", err)
			os.Exit(1)
		}

		// Approve DXP token transfer
		logger.Info("Approving DXP token transfer...")
		txHash, err := ethClient.ApproveDXPToken(amountWei)
		if err != nil {
			logger.Error("Failed to approve DXP token transfer: %v", err)
			os.Exit(1)
		}

		logger.Success("Approval transaction submitted: %s", txHash)
		logger.Info("Waiting for approval transaction to be mined...")

		// Wait for the approval transaction to be mined
		_, err = ethClient.WaitForTransaction(txHash)
		if err != nil {
			logger.Error("Failed to wait for approval transaction: %v", err)
			os.Exit(1)
		}

		logger.Success("Approval transaction mined successfully")

		// Register as a verifier with the specified farm ID
		logger.Info("Registering as a verifier with %d DXP tokens for Farm ID %d...", *registerAmount, *farmId)
		tx, err := ethClient.RegisterVerifierWithFarmID(amountWei, int64(*farmId))
		if err != nil {
			logger.Error("Failed to register as a verifier: %v", err)
			os.Exit(1)
		}

		logger.Success("Registration transaction submitted: %s", tx.Hash().Hex())
		logger.Info("Waiting for registration transaction to be mined...")

		// Wait for the registration transaction to be mined
		_, err = ethClient.WaitForTransaction(tx.Hash().Hex())
		if err != nil {
			logger.Error("Failed to wait for registration transaction: %v", err)
			os.Exit(1)
		}

		logger.Success("Successfully registered as a verifier!")
		logger.Info("Farm ID: %d", *farmId)
		logger.Info("Staked Amount: %d DXP", *registerAmount)

	case "status":
		// Parse status command flags
		statusCmd.Parse(os.Args[2:])

		// Initialize Ethereum client
		ethClient, err := eth.NewClient(rpcURL, privateKeyHex, contractAddress, tokenAddress)
		if err != nil {
			logger.Error("Failed to initialize Ethereum client: %v", err)
			os.Exit(1)
		}

		// Check if registered as verifier
		isRegistered, err := ethClient.IsRegisteredVerifier()
		if err != nil {
			logger.Error("Failed to check verifier status: %v", err)
			os.Exit(1)
		}

		if !isRegistered {
			fmt.Println("Not registered as a verifier")
			os.Exit(0)
		}

		// Get verifier stake
		stake, err := ethClient.GetVerifierStake()
		if err != nil {
			logger.Error("Failed to get verifier stake: %v", err)
			os.Exit(1)
		}

		// Get assigned farms
		farms, err := ethClient.GetAssignedFarms()
		if err != nil {
			logger.Error("Failed to get assigned farms: %v", err)
			os.Exit(1)
		}

		// Get verifier metrics
		metrics, err := ethClient.GetVerifierMetrics()
		if err != nil {
			logger.Error("Failed to get verifier metrics: %v", err)
			os.Exit(1)
		}

		// Calculate pending rewards
		pendingRewards, err := ethClient.CalculatePendingRewards()
		if err != nil {
			logger.Error("Failed to calculate pending rewards: %v", err)
			os.Exit(1)
		}

		// Print verifier status
		fmt.Println("=== Verifier Status ===")
		fmt.Printf("Wallet: %s\n", ethClient.GetWalletAddress())
		fmt.Printf("Registered: %t\n", isRegistered)
		fmt.Printf("Stake: %s DXP\n", ethClient.FormatTokenAmount(stake))
		
		// Print assigned farms
		fmt.Println("\n=== Assigned Farms ===")
		if len(farms) == 0 {
			fmt.Println("Not assigned to any farms")
		} else {
			for _, farmID := range farms {
				isActive, err := ethClient.IsVerifierActiveForFarm(farmID)
				status := "Registered"
				if err == nil && isActive {
					status = "Active"
				}
				fmt.Printf("Farm ID: %d (Status: %s)\n", farmID, status)
			}
		}
		
		// Print verifier metrics
		fmt.Println("\n=== Verifier Metrics ===")
		fmt.Printf("Verifications Performed: %d\n", metrics.VerificationsPerformed)
		fmt.Printf("Total Uptime: %s\n", formatDuration(metrics.TotalUptime))
		fmt.Printf("Last Active: %s\n", formatTime(metrics.LastActiveTimestamp))
		
		// Print rewards
		fmt.Println("\n=== Rewards ===")
		fmt.Printf("Pending Rewards: %s DXP\n", ethClient.FormatTokenAmount(pendingRewards))
		fmt.Printf("Last Claimed: %s\n", formatTime(metrics.LastRewardsClaim))
		
		// Calculate estimated daily rewards based on current metrics
		// This is a client-side calculation to show potential earnings
		dailyVerifications := float64(metrics.VerificationsPerformed)
		if metrics.TotalUptime > 0 {
			// Calculate verifications per day based on total uptime
			daysActive := float64(metrics.TotalUptime) / (24 * 60 * 60)
			if daysActive > 0 {
				dailyVerifications = float64(metrics.VerificationsPerformed) / daysActive
			}
		}
		
		// Show estimated daily earnings (simple calculation)
		fmt.Printf("Estimated Daily Verifications: %.2f\n", dailyVerifications)

	case "claim-rewards":
		// Parse claim-rewards command flags
		claimCmd.Parse(os.Args[2:])
		
		// Initialize Ethereum client
		ethClient, err := eth.NewClient(rpcURL, privateKeyHex, contractAddress, tokenAddress)
		if err != nil {
			logger.Error("Failed to initialize Ethereum client: %v", err)
			os.Exit(1)
		}
		
		// Check if registered as verifier
		isRegistered, err := ethClient.IsRegisteredVerifier()
		if err != nil {
			logger.Error("Failed to check verifier status: %v", err)
			os.Exit(1)
		}
		
		if !isRegistered {
			fmt.Println("Not registered as a verifier")
			os.Exit(0)
		}
		
		// Calculate pending rewards
		pendingRewards, err := ethClient.CalculatePendingRewards()
		if err != nil {
			logger.Error("Failed to calculate pending rewards: %v", err)
			os.Exit(1)
		}
		
		// Check if there are rewards to claim
		if pendingRewards.Cmp(big.NewInt(0)) <= 0 {
			logger.Info("No rewards to claim")
			os.Exit(0)
		}
		
		logger.Info("Claiming %s DXP rewards...", ethClient.FormatTokenAmount(pendingRewards))
		
		// Claim rewards
		tx, err := ethClient.ClaimRewards()
		if err != nil {
			logger.Error("Failed to claim rewards: %v", err)
			os.Exit(1)
		}
		
		txHash := tx.Hash().Hex()
		logger.Success("Claim transaction submitted: %s", txHash)
		logger.Info("Waiting for transaction to be mined...")
		
		// Wait for the transaction to be mined
		_, err = ethClient.WaitForTransaction(txHash)
		if err != nil {
			logger.Error("Failed to wait for transaction: %v", err)
			os.Exit(1)
		}
		
		logger.Success("Successfully claimed rewards!")

	case "withdraw":
		// Parse withdraw command flags
		withdrawCmd.Parse(os.Args[2:])

		// Validate required flags
		if *withdrawAmount <= 0 {
			logger.Error("Error: --amount flag is required and must be greater than 0")
			os.Exit(1)
		}

		// Initialize Ethereum client
		ethClient, err := eth.NewClient(rpcURL, privateKeyHex, contractAddress, tokenAddress)
		if err != nil {
			logger.Error("Failed to initialize Ethereum client: %v", err)
			os.Exit(1)
		}

		// Check if registered as verifier
		isRegistered, err := ethClient.IsRegisteredVerifier()
		if err != nil {
			logger.Error("Failed to check verifier status: %v", err)
			os.Exit(1)
		}

		if !isRegistered {
			fmt.Println("Not registered as a verifier")
			os.Exit(0)
		}

		// Get verifier stake
		stake, err := ethClient.GetVerifierStake()
		if err != nil {
			logger.Error("Failed to get verifier stake: %v", err)
			os.Exit(1)
		}

		// Convert amount to wei
		amountWei, err := ethClient.ConvertToWei(fmt.Sprintf("%d", *withdrawAmount))
		if err != nil {
			logger.Error("Failed to convert amount to wei: %v", err)
			os.Exit(1)
		}

		// Check if stake is sufficient
		if stake.Cmp(amountWei) < 0 {
			logger.Error("Insufficient stake. Requested: %s DXP, Available: %s DXP",
				ethClient.FormatTokenAmount(amountWei),
				ethClient.FormatTokenAmount(stake))
			os.Exit(1)
		}

		logger.Info("Withdrawing %s DXP from stake...", ethClient.FormatTokenAmount(amountWei))

		// Withdraw stake
		tx, err := ethClient.WithdrawVerifierStake(amountWei)
		if err != nil {
			logger.Error("Failed to withdraw stake: %v", err)
			os.Exit(1)
		}

		txHash := tx.Hash().Hex()
		logger.Success("Withdrawal transaction submitted: %s", txHash)

		// Wait for transaction confirmation
		logger.Info("Waiting for transaction confirmation...")
		_, err = ethClient.WaitForTransaction(txHash)
		if err != nil {
			logger.Error("Transaction failed: %v", err)
			os.Exit(1)
		}

		logger.Success("Stake withdrawn successfully!")

	case "send":
		// Check if key and value are provided
		if len(os.Args) < 4 {
			logger.Error("Error: send command requires key and value arguments")
			logger.Info("Usage: ./dxp-verifier send <key> <value>")
			os.Exit(1)
		}

		key := os.Args[2]
		value := os.Args[3]

		// Initialize P2P host
		host, err := p2p.NewHost()
		if err != nil {
			logger.Error("Failed to create P2P host: %v", err)
			os.Exit(1)
		}

		// Initialize the Dexponent protocol
		protocol := p2p.NewDexponentProtocol(host)

		// Wait a bit for peer discovery
		logger.Info("Waiting for peer discovery...")
		time.Sleep(5 * time.Second)

		// Get Dexponent peers
		peers := protocol.GetDexponentPeers()
		if len(peers) == 0 {
			logger.Info("No Dexponent peers found")
			os.Exit(1)
		}

		logger.Info("Sending data to %d Dexponent peers...", len(peers))
		for _, peerID := range peers {
			// Note: This is a placeholder. The actual implementation of SendData 
			// needs to be added to the DexponentProtocol
			logger.Info("Would send data to %s: key=%s, value=%s", peerID.String(), key, value)
			// Uncomment when SendData is implemented:
			// err := protocol.SendData(peerID, key, value)
			// if err != nil {
			//     logger.Error("Failed to send data to %s: %v", peerID.String(), err)
			// } else {
			//     logger.Success("Data sent to %s", peerID.String())
			// }
		}

	case "stop":
		// Parse stop command flags
		stopCmd.Parse(os.Args[2:])

		// Try to read PID from file
		pidFile := "verifier.pid"
		pidBytes, err := os.ReadFile(pidFile)
		if err != nil {
			logger.Error("Failed to read PID file: %v", err)
			logger.Info("Is the verifier running in detached mode?")
			os.Exit(1)
		}

		// Parse PID
		var pid int
		_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
		if err != nil {
			logger.Error("Failed to parse PID: %v", err)
			os.Exit(1)
		}

		// Send SIGTERM to process
		process, err := os.FindProcess(pid)
		if err != nil {
			logger.Error("Failed to find process: %v", err)
			os.Exit(1)
		}

		err = process.Signal(syscall.SIGTERM)
		if err != nil {
			logger.Error("Failed to send signal: %v", err)
			os.Exit(1)
		}

		logger.Success("Sent SIGTERM to process %d", pid)

		// Try to remove PID file
		err = os.Remove(pidFile)
		if err != nil {
			logger.Warn("Failed to remove PID file: %v", err)
		}

	default:
		logger.Error("Unknown command: %s", cmd)
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

// displayDexponentPeers periodically checks for peer changes and displays the list only when changes occur
func displayDexponentPeers(protocol *p2p.DexponentProtocol) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Track previous peer list for comparison
	var previousPeers []peer.ID

	for {
		select {
		case <-ticker.C:
			currentPeers := protocol.GetDexponentPeers()

			// Check if the peer list has changed
			if peersChanged(previousPeers, currentPeers) {
				logger.Info("Connected to %d Dexponent peers:", len(currentPeers))
				for _, peerID := range currentPeers {
					logger.Info("  Dexponent Peer: %s", peerID.String())
				}

				// Update previous peers
				previousPeers = make([]peer.ID, len(currentPeers))
				copy(previousPeers, currentPeers)
			}
		}
	}
}

// peersChanged checks if the peer lists are different
func peersChanged(previous, current []peer.ID) bool {
	if len(previous) != len(current) {
		return true
	}

	// Create maps for faster lookup
	prevMap := make(map[string]bool)
	for _, p := range previous {
		prevMap[p.String()] = true
	}

	// Check if any current peer is not in the previous list
	for _, p := range current {
		if !prevMap[p.String()] {
			return true
		}
	}

	return false
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

// runBenchmarkUpdater periodically updates benchmarks for farms that the verifier is active for
func runBenchmarkUpdater(ethClient *eth.Client, farms []int64, interval time.Duration) {
	// Wait for initial setup
	time.Sleep(15 * time.Second)
	
	// Log that the benchmark updater is running
	intervalStr := fmt.Sprintf("Benchmark updater will run every %s", interval.String())
	logger.Success(intervalStr)
	
	// Track pending transactions
	pendingTxs := make(map[common.Hash]time.Time)
	
	// Start a goroutine to check for transaction confirmations
	go func() {
		for {
			// Check each pending transaction
			for txHash, submitTime := range pendingTxs {
				// Skip if transaction is less than 10 seconds old
				if time.Since(submitTime) < 10*time.Second {
					continue
				}
				
				// Check if transaction is confirmed
				receipt, err := ethClient.GetTransactionReceipt(txHash)
				if err != nil {
					logger.Warn("Failed to get receipt for transaction %s: %v", txHash.Hex(), err)
					continue
				}
				
				// If transaction is confirmed, remove it from pending list
				if receipt != nil {
					if receipt.Status == 1 {
						logger.Success("Benchmark update transaction %s confirmed successfully", txHash.Hex())
					} else {
						logger.Error("Benchmark update transaction %s failed", txHash.Hex())
					}
					delete(pendingTxs, txHash)
				}
			}
			
			time.Sleep(5 * time.Second)
		}
	}()
	
	// Run benchmark updater loop
	for {
		// Update benchmark for each farm
		for _, farmID := range farms {
			// Check if verifier is still active for this farm
			isActive, err := ethClient.IsVerifierActiveForFarm(farmID)
			if err != nil {
				logger.Error("Failed to check if verifier is active for farm %d: %v", farmID, err)
				continue
			}
			
			if !isActive {
				logger.Warn("Verifier is no longer active for farm %d, skipping benchmark update", farmID)
				continue
			}
			
			// Get current benchmark
			farmData, err := ethClient.GetFarmData(farmID)
			if err != nil {
				logger.Error("Failed to get farm data for farm %d: %v", farmID, err)
				continue
			}
			
			currentBenchmark := farmData.Benchmark
			
			// Calculate new benchmark with small random variation (+/- 1-4%)
			// Convert from basis points to percentage for easier calculation
			currentPct := float64(currentBenchmark) / 100.0
			
			// If current benchmark is 0, start with 10%
			if currentBenchmark == 0 {
				currentPct = 10.0
			}
			
			// Random variation between -4% and +4% of the current value
			variation := (rand.Float64()*8.0 - 4.0) / 100.0
			newPct := currentPct * (1.0 + variation)
			
			// Ensure it stays within reasonable bounds (5-15%)
			if newPct < 5.0 {
				newPct = 5.0
			} else if newPct > 15.0 {
				newPct = 15.0
			}
			
			// Convert back to basis points
			newBenchmark := uint64(newPct * 100.0)
			
			// Only update if the benchmark has changed
			if uint64(currentBenchmark) != newBenchmark {
				logger.Success("Updating benchmark for farm %d: %.2f%% -> %.2f%%", 
					farmID, float64(currentBenchmark)/100.0, newPct)
				
				// Submit new benchmark
				tx, err := ethClient.SetFarmBenchmark(farmID, big.NewInt(int64(newBenchmark)))
				if err != nil {
					logger.Error("Failed to update benchmark for farm %d: %v", farmID, err)
					continue
				}
				
				txHash := tx.Hash()
				logger.Success("Benchmark update transaction submitted: %s", txHash.Hex())
				
				// Add to pending transactions map
				pendingTxs[txHash] = time.Now()
			}
		}
		
		// Wait for next interval
		time.Sleep(interval)
	}
}

// formatDuration formats a duration in seconds as a human-readable string
func formatDuration(seconds uint64) string {
	duration := time.Duration(seconds) * time.Second
	
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// formatTime formats a Unix timestamp as a human-readable string
func formatTime(timestamp uint64) string {
	if timestamp == 0 {
		return "Never"
	}
	
	t := time.Unix(int64(timestamp), 0)
	return t.Format("2006-01-02 15:04:05")
}
