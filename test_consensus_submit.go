package main

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/dexponent/dxp-verifier/pkg/eth"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		os.Exit(1)
	}

	// Get Ethereum configuration from environment variables
	rpcURL := os.Getenv("BASE_RPC_URL")
	privateKeyHex := os.Getenv("WALLET_PRIVATE_KEY")
	protocolAddress := os.Getenv("PROTOCOL_CORE_ADDRESS")
	consensusAddress := os.Getenv("CONSENSUS_ADDRESS")
	tokenAddress := os.Getenv("DXP_TOKEN_ADDRESS")

	// Validate required environment variables
	if rpcURL == "" || privateKeyHex == "" || protocolAddress == "" || consensusAddress == "" {
		fmt.Println("Error: Required environment variables not set. Check .env file.")
		os.Exit(1)
	}

	// Initialize Ethereum client
	ethClient, err := eth.NewClient(rpcURL, privateKeyHex, protocolAddress, consensusAddress, tokenAddress)
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

	if !isRegistered {
		fmt.Println("Not registered as a verifier. Registering now...")
		
		// Convert amount to wei
		amountWei, err := ethClient.ConvertToWei("100")
		if err != nil {
			fmt.Printf("Failed to convert amount to wei: %v\n", err)
			os.Exit(1)
		}

		// Approve token transfer
		txHash, err := ethClient.ApproveDXPToken(amountWei)
		if err != nil {
			fmt.Printf("Failed to approve token transfer: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Approval transaction submitted: %s\n", txHash)
		fmt.Println("Waiting for approval transaction to be mined...")
		
		// Wait for approval transaction to be mined
		_, err = ethClient.WaitForTransaction(txHash)
		if err != nil {
			fmt.Printf("Failed to wait for approval transaction: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Approval transaction mined successfully")

		// Register as verifier
		farmID := int64(1)
		tx, err := ethClient.RegisterVerifierWithFarmID(amountWei, farmID)
		if err != nil {
			fmt.Printf("Failed to register as verifier: %v\n", err)
			os.Exit(1)
		}

		txHash = tx.Hash().Hex()
		fmt.Printf("Registration transaction submitted: %s\n", txHash)
		fmt.Println("Waiting for registration transaction to be mined...")

		// Wait for registration transaction to be mined
		_, err = ethClient.WaitForTransaction(txHash)
		if err != nil {
			fmt.Printf("Failed to wait for registration transaction: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Successfully registered as a verifier!")
		
		// Wait a moment to ensure registration is fully processed
		time.Sleep(2 * time.Second)
	}

	// Get assigned farms
	farms, err := ethClient.GetAssignedFarms()
	if err != nil {
		fmt.Printf("Failed to get assigned farms: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Verifier Status ===")
	fmt.Printf("Wallet: %s\n", ethClient.GetWalletAddress())
	fmt.Printf("Registered: %t\n", isRegistered)

	// Print assigned farms
	fmt.Println("\n=== Assigned Farms ===")
	if len(farms) == 0 {
		fmt.Println("Not assigned to any farms")
		os.Exit(1)
	} else {
		fmt.Print("Assigned to farm IDs: ")
		for i, farmID := range farms {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(farmID)
		}
		fmt.Println()
	}

	// Now test the SubmitVerification function with both score and benchmark
	farmID := farms[0] // Use the first assigned farm
	// Score should be a value between 0-1
	// Benchmark should be a percentage between 0-100
	score := big.NewFloat(0.75)   // 0.75 score (75%)
	benchmark := big.NewFloat(80) // 80% benchmark

	fmt.Printf("\n=== Starting a New Consensus Round ===\n")
	fmt.Printf("Attempting to start a consensus round for farm ID %d...\n", farmID)
	
	// Start a new consensus round
	roundTx, roundErr := ethClient.StartConsensusRound(farmID)
	if roundErr != nil {
		fmt.Printf("Failed to start consensus round: %v\n", roundErr)
		fmt.Println("This is expected if you're not the contract owner. Proceeding anyway...")
	} else {
		txHash := roundTx.Hash().Hex()
		fmt.Printf("Round start transaction submitted: %s\n", txHash)
		fmt.Println("Waiting for round start transaction to be mined...")
		
		// Wait for transaction to be mined
		receipt, err := ethClient.WaitForTransaction(txHash)
		if err != nil {
			fmt.Printf("Failed to wait for round start transaction: %v\n", err)
		} else if receipt.Status == 1 {
			fmt.Println("✅ Consensus round successfully started!")
		} else {
			fmt.Println("❌ Round start transaction failed!")
		}
		
		// Wait a moment to ensure the round is fully started
		time.Sleep(2 * time.Second)
	}
	
	fmt.Printf("\n=== Submitting Verification ===")
	fmt.Printf("\nFarm ID: %d\n", farmID)
	fmt.Printf("Score: %.2f\n", score)
	fmt.Printf("Benchmark: %.2f\n", benchmark)

	// Submit verification
	var txHash string
	var alreadySubmitted bool
	
	tx, err := ethClient.SubmitVerification(farmID, score, benchmark)
	if err != nil {
		// Check if the error is 'Already submitted'
		if strings.Contains(err.Error(), "Already submitted") {
			fmt.Println("⚠️ Verification already submitted for this round. Proceeding with finalization...")
			alreadySubmitted = true
			// Continue with the rest of the script
		} else {
			fmt.Printf("Failed to submit verification: %v\n", err)
			os.Exit(1)
		}
	}

	if !alreadySubmitted {
		txHash = tx.Hash().Hex()
		fmt.Printf("Verification transaction submitted: %s\n", txHash)
		fmt.Println("Waiting for verification transaction to be mined...")

		// Wait for verification transaction to be mined
		receipt, err := ethClient.WaitForTransaction(txHash)
		if err != nil {
			fmt.Printf("Failed to wait for verification transaction: %v\n", err)
			os.Exit(1)
		}

		if receipt.Status == 1 {
			fmt.Println("✅ Verification successfully submitted!")
		} else {
			fmt.Println("❌ Verification transaction failed!")
			os.Exit(1)
		}
	}

	// Finalize the consensus round (requires owner access)
	fmt.Printf("\n=== Finalizing Consensus Round ===\n")
	fmt.Printf("Attempting to finalize consensus round for farm ID %d...\n", farmID)
	
	// Finalize the consensus round
	finalizeRoundTx, finalizeErr := ethClient.FinalizeConsensusRound(farmID)
	if finalizeErr != nil {
		fmt.Printf("Failed to finalize consensus round: %v\n", finalizeErr)
		fmt.Println("This is expected if you're not the contract owner. Proceeding anyway...")
	} else {
		txHash := finalizeRoundTx.Hash().Hex()
		fmt.Printf("Round finalization transaction submitted: %s\n", txHash)
		fmt.Println("Waiting for round finalization transaction to be mined...")
		
		// Wait for transaction to be mined
		receipt, err := ethClient.WaitForTransaction(txHash)
		if err != nil {
			fmt.Printf("Failed to wait for round finalization transaction: %v\n", err)
		} else if receipt.Status == 1 {
			fmt.Println("✅ Consensus round successfully finalized!")
		} else {
			fmt.Println("❌ Round finalization transaction failed!")
		}
	}

	// Withdraw verifier stake (optional cleanup step)
	fmt.Printf("\n=== Withdrawing Verifier Stake ===\n")
	fmt.Printf("Checking current stake for farm ID %d...\n", farmID)
	
	// Check current stake
	stake, err := ethClient.GetVerifierStake(farmID)
	if err != nil {
		fmt.Printf("Failed to get verifier stake: %v\n", err)
	} else {
		fmt.Printf("Current stake: %s DXP\n", stake.String())
		
		if stake.Cmp(big.NewInt(0)) > 0 {
			fmt.Println("Attempting to withdraw stake...")
			
			// Withdraw all stake
			withdrawTx, withdrawErr := ethClient.WithdrawVerifierStake(farmID, stake)
			if withdrawErr != nil {
				fmt.Printf("Failed to withdraw stake: %v\n", withdrawErr)
			} else {
				txHash := withdrawTx.Hash().Hex()
				fmt.Printf("Withdrawal transaction submitted: %s\n", txHash)
				fmt.Println("Waiting for withdrawal transaction to be mined...")
				
				// Wait for transaction to be mined
				receipt, err := ethClient.WaitForTransaction(txHash)
				if err != nil {
					fmt.Printf("Failed to wait for withdrawal transaction: %v\n", err)
				} else if receipt.Status == 1 {
					fmt.Println("✅ Stake successfully withdrawn!")
				} else {
					fmt.Println("❌ Withdrawal transaction failed!")
				}
			}
		} else {
			fmt.Println("No stake to withdraw.")
		}
	}
}
