package eth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

// Client represents an Ethereum client for interacting with the DXP contract
type Client struct {
	ethClient      *ethclient.Client
	contractAddr   common.Address
	privateKey     *ecdsa.PrivateKey
	contractABI    abi.ABI
	contractBound  *bind.BoundContract
	chainID        *big.Int
	gasLimit       uint64
	gasPriceMulti  float64
	walletAddress  common.Address
}

// NewClient creates a new Ethereum client
func NewClient() (*Client, error) {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Warning: .env file not found, using environment variables\n")
	}

	// Get RPC URL
	rpcURL := os.Getenv("BASE_RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("BASE_RPC_URL not set in environment")
	}

	// Connect to Ethereum node
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %v", err)
	}

	// Get contract address
	contractAddrStr := os.Getenv("DXP_CONTRACT_ADDRESS")
	if contractAddrStr == "" {
		return nil, fmt.Errorf("DXP_CONTRACT_ADDRESS not set in environment")
	}
	contractAddr := common.HexToAddress(contractAddrStr)

	// Get private key
	privKeyStr := os.Getenv("WALLET_PRIVATE_KEY")
	if privKeyStr == "" {
		return nil, fmt.Errorf("WALLET_PRIVATE_KEY not set in environment")
	}
	if strings.HasPrefix(privKeyStr, "0x") {
		privKeyStr = privKeyStr[2:]
	}
	privateKey, err := crypto.HexToECDSA(privKeyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %v", err)
	}

	// Get wallet address from private key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}
	walletAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// Get chain ID
	chainIDStr := os.Getenv("CHAIN_ID")
	chainID := big.NewInt(11155111) // Default to Sepolia
	if chainIDStr != "" {
		chainID, ok = new(big.Int).SetString(chainIDStr, 10)
		if !ok {
			return nil, fmt.Errorf("invalid CHAIN_ID: %s", chainIDStr)
		}
	}

	// Get gas limit
	gasLimit := uint64(3000000) // Default gas limit
	gasLimitStr := os.Getenv("GAS_LIMIT")
	if gasLimitStr != "" {
		gasLimitBig, ok := new(big.Int).SetString(gasLimitStr, 10)
		if !ok {
			return nil, fmt.Errorf("invalid GAS_LIMIT: %s", gasLimitStr)
		}
		gasLimit = gasLimitBig.Uint64()
	}

	// Get gas price multiplier
	gasPriceMulti := 1.1 // Default multiplier
	gasPriceMultiStr := os.Getenv("GAS_PRICE_MULTIPLIER")
	if gasPriceMultiStr != "" {
		_, err := fmt.Sscanf(gasPriceMultiStr, "%f", &gasPriceMulti)
		if err != nil {
			return nil, fmt.Errorf("invalid GAS_PRICE_MULTIPLIER: %s", gasPriceMultiStr)
		}
	}

	// Load contract ABI
	contractABI, err := abi.JSON(strings.NewReader(DexponentProtocolABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %v", err)
	}

	// Create bound contract
	contractBound := bind.NewBoundContract(contractAddr, contractABI, ethClient, ethClient, ethClient)

	return &Client{
		ethClient:     ethClient,
		contractAddr:  contractAddr,
		privateKey:    privateKey,
		contractABI:   contractABI,
		contractBound: contractBound,
		chainID:       chainID,
		gasLimit:      gasLimit,
		gasPriceMulti: gasPriceMulti,
		walletAddress: walletAddress,
	}, nil
}

// RegisterVerifier registers the current wallet as a verifier
func (c *Client) RegisterVerifier() (string, error) {
	fmt.Printf("Registering wallet %s as a verifier...\n", c.walletAddress.Hex())

	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return "", err
	}

	// Create transaction data for registerVerifier function
	data, err := c.contractABI.Pack("registerVerifier")
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %v", err)
	}

	// Send transaction
	tx, err := c.sendTransaction(auth, data)
	if err != nil {
		return "", err
	}

	fmt.Printf("Transaction sent: %s\n", tx.Hash().Hex())
	return tx.Hash().Hex(), nil
}

// SubmitConsensusResult submits the consensus result to the DXP contract
func (c *Client) SubmitConsensusResult(farmId int64, score float64, participants []string) (string, error) {
	fmt.Printf("Submitting consensus result for farm %d: score %.4f with %d participants\n", 
		farmId, score, len(participants))

	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return "", err
	}

	// Convert participants to addresses
	participantAddrs := make([]common.Address, len(participants))
	for i, p := range participants {
		participantAddrs[i] = common.HexToAddress(p)
	}

	// Create transaction data for submitVerification function
	scoreBig := new(big.Int)
	scoreBig.SetInt64(int64(score * 10000)) // Convert to fixed point with 4 decimal places

	data, err := c.contractABI.Pack("submitVerification", big.NewInt(farmId), scoreBig, participantAddrs)
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %v", err)
	}

	// Send transaction
	tx, err := c.sendTransaction(auth, data)
	if err != nil {
		return "", err
	}

	fmt.Printf("Transaction sent: %s\n", tx.Hash().Hex())
	return tx.Hash().Hex(), nil
}

// GetPendingRewards returns the pending rewards for the verifier
func (c *Client) GetPendingRewards() (*big.Int, error) {
	fmt.Printf("Checking pending rewards for %s...\n", c.walletAddress.Hex())

	// Create call options
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}

	// Call getPendingRewards function
	var result *big.Int
	var out []interface{}
	err := c.contractBound.Call(callOpts, &out, "getPendingRewards", c.walletAddress)
	if err == nil && len(out) > 0 {
		result = out[0].(*big.Int)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending rewards: %v", err)
	}

	return result, nil
}

// ClaimRewards claims the pending rewards for the verifier
func (c *Client) ClaimRewards() (string, error) {
	fmt.Printf("Claiming rewards for %s...\n", c.walletAddress.Hex())

	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return "", err
	}

	// Create transaction data for claimRewards function
	data, err := c.contractABI.Pack("claimRewards")
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %v", err)
	}

	// Send transaction
	tx, err := c.sendTransaction(auth, data)
	if err != nil {
		return "", err
	}

	fmt.Printf("Transaction sent: %s\n", tx.Hash().Hex())
	return tx.Hash().Hex(), nil
}

// IsRegisteredVerifier checks if the current wallet is registered as a verifier
func (c *Client) IsRegisteredVerifier() (bool, error) {
	fmt.Printf("Checking if %s is registered as a verifier...\n", c.walletAddress.Hex())

	// Create call options
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}

	// Call registeredVerifiers function
	var result bool
	var out []interface{}
	err := c.contractBound.Call(callOpts, &out, "registeredVerifiers", c.walletAddress)
	if err == nil && len(out) > 0 {
		result = out[0].(bool)
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if registered: %v", err)
	}

	return result, nil
}

// GetDXPBalance returns the DXP token balance of the wallet
func (c *Client) GetDXPBalance() (*big.Int, error) {
	// DXP token address from .env
	dxpTokenAddress := os.Getenv("DXP_TOKEN_ADDRESS")
	if dxpTokenAddress == "" {
		dxpTokenAddress = "0x08Eb4d9c6e388777b21b517A13030a08e39AC279" // Default DXP token address on Sepolia
	}

	// Create token contract instance
	tokenAddress := common.HexToAddress(dxpTokenAddress)
	tokenABI := `[{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(tokenABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ABI: %v", err)
	}

	tokenContract := bind.NewBoundContract(tokenAddress, parsedABI, c.ethClient, c.ethClient, c.ethClient)

	// Call balanceOf function
	var out []interface{}
	callOpts := &bind.CallOpts{Context: context.Background()}
	err = tokenContract.Call(callOpts, &out, "balanceOf", c.walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get DXP balance: %v", err)
	}

	if len(out) == 0 {
		return big.NewInt(0), nil
	}

	balance, ok := out[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to convert balance to big.Int")
	}

	return balance, nil
}

// GetDXPAllowance returns the DXP token allowance for the DXP contract
func (c *Client) GetDXPAllowance() (*big.Int, error) {
	// DXP token address from .env
	dxpTokenAddress := os.Getenv("DXP_TOKEN_ADDRESS")
	if dxpTokenAddress == "" {
		dxpTokenAddress = "0x08Eb4d9c6e388777b21b517A13030a08e39AC279" // Default DXP token address on Sepolia
	}

	// Create token contract instance
	tokenAddress := common.HexToAddress(dxpTokenAddress)
	tokenABI := `[{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(tokenABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ABI: %v", err)
	}

	tokenContract := bind.NewBoundContract(tokenAddress, parsedABI, c.ethClient, c.ethClient, c.ethClient)

	// Call allowance function
	var out []interface{}
	callOpts := &bind.CallOpts{Context: context.Background()}
	err = tokenContract.Call(callOpts, &out, "allowance", c.walletAddress, c.contractAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get DXP allowance: %v", err)
	}

	if len(out) == 0 {
		return big.NewInt(0), nil
	}

	allowance, ok := out[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to convert allowance to big.Int")
	}

	return allowance, nil
}

// GetCurrentBlock returns the current block number
func (c *Client) GetCurrentBlock() (uint64, error) {
	blockNumber, err := c.ethClient.BlockNumber(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get current block number: %v", err)
	}

	return blockNumber, nil
}

// GetVerifierStatus returns the status of the verifier
func (c *Client) GetVerifierStatus() (map[string]interface{}, error) {
	// Check if registered
	registered, err := c.IsRegisteredVerifier()
	if err != nil {
		return nil, err
	}

	// Get pending rewards if registered
	var rewards *big.Int
	if registered {
		rewards, err = c.GetPendingRewards()
		if err != nil {
			return nil, err
		}
	}

	// Get ETH balance
	balance, err := c.ethClient.BalanceAt(context.Background(), c.walletAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH balance: %v", err)
	}

	// Get DXP balance
	dxpBalance, err := c.GetDXPBalance()
	if err != nil {
		dxpBalance = big.NewInt(0)
	}

	// Get DXP allowance
	dxpAllowance, err := c.GetDXPAllowance()
	if err != nil {
		dxpAllowance = big.NewInt(0)
	}

	// Return status
	return map[string]interface{}{
		"address":       c.walletAddress.Hex(),
		"registered":    registered,
		"rewards":       rewards,
		"ethBalance":    balance,
		"dxpBalance":    dxpBalance,
		"dxpAllowance":  dxpAllowance,
	}, nil
}

// Helper function to create transaction options
func (c *Client) createTransactOpts() (*bind.TransactOpts, error) {
	// Get nonce
	nonce, err := c.ethClient.PendingNonceAt(context.Background(), c.walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %v", err)
	}

	// Get gas price
	gasPrice, err := c.ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %v", err)
	}

	// Apply gas price multiplier
	gasPriceFloat := new(big.Float).SetInt(gasPrice)
	gasPriceFloat.Mul(gasPriceFloat, big.NewFloat(c.gasPriceMulti))
	gasPriceInt, _ := gasPriceFloat.Int(nil)

	// Create transaction options
	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction signer: %v", err)
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // No ETH sent
	auth.GasLimit = c.gasLimit      // Gas limit
	auth.GasPrice = gasPriceInt     // Gas price with multiplier

	return auth, nil
}

// Helper function to send a transaction
func (c *Client) sendTransaction(auth *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	// Create transaction
	// Make sure GasTipCap is not higher than GasFeeCap
	gasTipCap := big.NewInt(1000000000) // 1 gwei tip
	gasFeeCap := auth.GasPrice
	
	// If GasTipCap is higher than GasFeeCap, set it to 1/10 of GasFeeCap
	if gasTipCap.Cmp(gasFeeCap) > 0 {
		gasTipCap = new(big.Int).Div(gasFeeCap, big.NewInt(10))
	}
	
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   c.chainID,
		Nonce:     auth.Nonce.Uint64(),
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       auth.GasLimit,
		To:        &c.contractAddr,
		Value:     auth.Value,
		Data:      data,
	})

	// Sign transaction
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Send transaction
	err = c.ethClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}

	return signedTx, nil
}

// WaitForTransaction waits for a transaction to be mined
func (c *Client) WaitForTransaction(txHash string) (*types.Receipt, error) {
	fmt.Printf("Waiting for transaction %s to be mined...\n", txHash)

	hash := common.HexToHash(txHash)
	for i := 0; i < 60; i++ { // Wait up to 5 minutes (60 * 5 seconds)
		receipt, err := c.ethClient.TransactionReceipt(context.Background(), hash)
		if err == nil {
			fmt.Printf("Transaction mined in block %d\n", receipt.BlockNumber)
			return receipt, nil
		} else if err != ethereum.NotFound {
			return nil, fmt.Errorf("failed to get transaction receipt: %v", err)
		}

		fmt.Printf("Transaction not yet mined, waiting 5 seconds...\n")
		time.Sleep(5 * time.Second)
	}

	return nil, fmt.Errorf("transaction not mined within timeout")
}
