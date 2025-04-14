package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/core/types"
)

// Amount of DXP tokens to fund (100 DXP)
const FundAmount = 100

const DexponentProtocolABI = `
[
	{
		"constant": false,
		"inputs": [
			{
				"name": "amount",
				"type": "uint256"
			}
		],
		"name": "fundRewardPool",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "spender",
				"type": "address"
			},
			{
				"name": "amount",
				"type": "uint256"
			}
		],
		"name": "approve",
		"outputs": [
			{
				"name": "",
				"type": "bool"
			}
		],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]
`

func main() {
	fmt.Println("DXP Verifier - Reward Pool Funding Tool (OWNER ONLY)")
	fmt.Println("====================================================")
	fmt.Println("This tool is intended for contract owners only.")
	fmt.Println("It will fund the reward pool with 100 DXP tokens.")
	fmt.Println()

	// Get environment variables
	rpcURL := os.Getenv("ETH_RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://sepolia.infura.io/v3/YOUR_INFURA_KEY" // Replace with your Infura key
		fmt.Println("Warning: Using default RPC URL. Set ETH_RPC_URL environment variable for custom URL.")
	}

	privateKeyHex := os.Getenv("ETH_PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Fatal("ETH_PRIVATE_KEY environment variable not set")
	}

	contractAddressHex := os.Getenv("CONTRACT_ADDRESS")
	if contractAddressHex == "" {
		log.Fatal("CONTRACT_ADDRESS environment variable not set")
	}

	tokenAddressHex := os.Getenv("TOKEN_ADDRESS")
	if tokenAddressHex == "" {
		tokenAddressHex = "0x08Eb4d9c6e388777b21b517A13030a08e39AC279" // Default DXP token address
		fmt.Println("Using default DXP token address. Set TOKEN_ADDRESS environment variable for custom address.")
	}

	// Connect to Ethereum node
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum node: %v", err)
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	// Get wallet address
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	fmt.Printf("Wallet address: %s\n", address.Hex())
	fmt.Println("IMPORTANT: This address must be the contract owner to succeed!")
	fmt.Println()

	// Get chain ID
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}

	// Convert fund amount to wei (1 DXP = 10^18 wei)
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	fundAmountWei := new(big.Int).Mul(big.NewInt(FundAmount), multiplier)
	fmt.Printf("Funding amount: %s DXP (%s wei)\n", big.NewInt(FundAmount).String(), fundAmountWei.String())

	// First, approve the contract to spend tokens
	contractAddress := common.HexToAddress(contractAddressHex)
	tokenAddress := common.HexToAddress(tokenAddressHex)

	// Parse ABI
	tokenAbi, err := abi.JSON(strings.NewReader(DexponentProtocolABI))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	// Pack parameters for approve
	data, err := tokenAbi.Pack("approve", contractAddress, fundAmountWei)
	if err != nil {
		log.Fatalf("Failed to pack parameters: %v", err)
	}

	// Create approve transaction
	approveTx, err := createAndSignTransaction(client, privateKey, chainID, tokenAddress, big.NewInt(0), data)
	if err != nil {
		log.Fatalf("Failed to create approve transaction: %v", err)
	}

	// Send approve transaction
	err = client.SendTransaction(context.Background(), approveTx)
	if err != nil {
		log.Fatalf("Failed to send approve transaction: %v", err)
	}

	fmt.Printf("Approve transaction sent: %s\n", approveTx.Hash().Hex())
	fmt.Println("Waiting for approve transaction to be mined...")

	// Wait for approve transaction to be mined
	receipt, err := waitForTransaction(client, approveTx.Hash())
	if err != nil {
		log.Fatalf("Failed to wait for approve transaction: %v", err)
	}

	if receipt.Status == 0 {
		log.Fatal("Approve transaction failed")
	}

	fmt.Println("Approve transaction confirmed!")

	// Now fund the reward pool using the ABI
	contractAbi, err := abi.JSON(strings.NewReader(DexponentProtocolABI))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	// Pack the parameters using the ABI
	data, err = contractAbi.Pack("fundRewardPool", fundAmountWei)
	if err != nil {
		log.Fatalf("Failed to pack parameters: %v", err)
	}

	// Create fundRewardPool transaction
	fundTx, err := createAndSignTransaction(client, privateKey, chainID, contractAddress, big.NewInt(0), data)
	if err != nil {
		log.Fatalf("Failed to create fund transaction: %v", err)
	}

	// Send fundRewardPool transaction
	err = client.SendTransaction(context.Background(), fundTx)
	if err != nil {
		log.Fatalf("Failed to send fund transaction: %v", err)
	}

	fmt.Printf("Fund transaction sent: %s\n", fundTx.Hash().Hex())
	fmt.Println("Waiting for fund transaction to be mined...")

	// Wait for fund transaction to be mined
	receipt, err = waitForTransaction(client, fundTx.Hash())
	if err != nil {
		log.Fatalf("Failed to wait for fund transaction: %v", err)
	}

	if receipt.Status == 0 {
		log.Fatal("Fund transaction failed")
	}

	fmt.Println("Success! Reward pool funded with 100 DXP")
}

// Helper function to create and sign a transaction
func createAndSignTransaction(client *ethclient.Client, privateKey *ecdsa.PrivateKey, chainID *big.Int, to common.Address, value *big.Int, data []byte) (*types.Transaction, error) {
	// Get the current nonce
	nonce, err := client.PendingNonceAt(context.Background(), crypto.PubkeyToAddress(privateKey.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %v", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %v", err)
	}

	// Apply gas price multiplier (2.0)
	multiplier := 2.0
	adjustedGasPrice := new(big.Int).Mul(gasPrice, big.NewInt(int64(multiplier*100)))
	adjustedGasPrice = new(big.Int).Div(adjustedGasPrice, big.NewInt(100))

	// Set gas limit
	gasLimit := uint64(500000)

	// Create the transaction
	tx := types.NewTransaction(nonce, to, value, gasLimit, adjustedGasPrice, data)

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	return signedTx, nil
}

// Helper function to wait for a transaction to be mined
func waitForTransaction(client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	for i := 0; i < 30; i++ {
		receipt, err := client.TransactionReceipt(context.Background(), txHash)
		if err == nil {
			return receipt, nil
		}

		// Wait for 2 seconds before checking again
		fmt.Println("Waiting for transaction to be mined...")
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("transaction not mined after 60 seconds")
}
