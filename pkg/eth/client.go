package eth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Default token address for Sepolia testnet
const DefaultDXPTokenAddress = "0x4ed7c70F96B99c776995fB64377f0d4aB3B0e1C1"

// VerifierMetrics represents the metrics of a verifier
type VerifierMetrics struct {
	VerificationsPerformed int64
	LastActiveTimestamp    uint64
	TotalUptime            uint64
	AccumulatedRewards     *big.Int
	LastRewardsClaim       uint64
}

// FarmData represents data about a farm
type FarmData struct {
	Score      uint64
	Benchmark  uint64
	LastUpdate uint64
}

// Client represents an Ethereum client
type Client struct {
	ethClient       *ethclient.Client
	privateKey      *ecdsa.PrivateKey
	contractAddress common.Address
	tokenAddress    common.Address
	chainID         *big.Int
	address         common.Address
}

// NewClient creates a new Ethereum client
func NewClient(rpcURL, privateKeyHex, contractAddress, tokenAddress string) (*Client, error) {
	// Connect to Ethereum node
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %v", err)
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Get chain ID
	chainID, err := ethClient.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Use default token address if not provided
	if tokenAddress == "" {
		tokenAddress = DefaultDXPTokenAddress
	}

	return &Client{
		ethClient:       ethClient,
		privateKey:      privateKey,
		contractAddress: common.HexToAddress(contractAddress),
		tokenAddress:    common.HexToAddress(tokenAddress),
		chainID:         chainID,
		address:         crypto.PubkeyToAddress(privateKey.PublicKey),
	}, nil
}

// GetAddress returns the Ethereum address of the client
func (c *Client) GetAddress() common.Address {
	return c.address
}

// GetDXPBalance returns the DXP token balance of the client
func (c *Client) GetDXPBalance() (*big.Int, error) {
	// Create the method signature for balanceOf
	methodSig := []byte("balanceOf(address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the address parameter
	address := c.GetAddress()
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	
	// Create the call data
	data := append(methodID, paddedAddress...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.tokenAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call balanceOf: %v", err)
	}
	
	// Parse the result
	balance := new(big.Int).SetBytes(result)
	return balance, nil
}

// ConvertToWei converts a string amount to wei
func (c *Client) ConvertToWei(amount string) (*big.Int, error) {
	// Parse the amount
	amountFloat, ok := new(big.Float).SetString(amount)
	if !ok {
		return nil, fmt.Errorf("failed to parse amount: %s", amount)
	}

	// Convert to wei (1 DXP = 10^18 wei)
	multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	result := new(big.Float).Mul(amountFloat, multiplier)

	// Convert to big.Int
	amountWei := new(big.Int)
	result.Int(amountWei)

	return amountWei, nil
}

// ConvertFromWei converts wei to a string amount
func (c *Client) ConvertFromWei(wei *big.Int) string {
	// Convert to big.Float
	weiFloat := new(big.Float).SetInt(wei)

	// Convert from wei (1 DXP = 10^18 wei)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	result := new(big.Float).Quo(weiFloat, divisor)

	// Convert to string with 6 decimal places
	return result.Text('f', 6)
}

// FormatTokenAmount formats a token amount for display
func (c *Client) FormatTokenAmount(wei *big.Int) string {
	return c.ConvertFromWei(wei)
}

// ApproveDXPToken approves the contract to spend DXP tokens
func (c *Client) ApproveDXPToken(amount *big.Int) (string, error) {
	// Create the method signature for approve
	methodSig := []byte("approve(address,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedAddress := common.LeftPadBytes(c.contractAddress.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)
	
	// Create the transaction data
	data := append(methodID, append(paddedAddress, paddedAmount...)...)
	
	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.tokenAddress, big.NewInt(0), data)
	if err != nil {
		return "", fmt.Errorf("failed to create and sign transaction: %v", err)
	}
	
	// Send the transaction
	err = c.ethClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}
	
	return tx.Hash().Hex(), nil
}

// RegisterVerifierWithFarmID registers the current wallet as a verifier with a farm ID
func (c *Client) RegisterVerifierWithFarmID(stakeAmount *big.Int, farmID int64) (*types.Transaction, error) {
	// Create the method signature for registerVerifier
	methodSig := []byte("registerVerifier(address,uint256,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	address := c.GetAddress()
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(stakeAmount.Bytes(), 32)
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	
	// Create the transaction data
	data := append(methodID, append(paddedAddress, append(paddedAmount, paddedFarmID...)...)...)
	
	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.contractAddress, big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("failed to create and sign transaction: %v", err)
	}
	
	// Send the transaction
	err = c.ethClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	
	return tx, nil
}

// IsRegisteredVerifier checks if the current wallet is registered as a verifier
func (c *Client) IsRegisteredVerifier() (bool, error) {
	// Create the method signature for isVerifier
	methodSig := []byte("isVerifier(address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the address parameter
	address := c.GetAddress()
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	
	// Create the call data
	data := append(methodID, paddedAddress...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return false, fmt.Errorf("failed to call isVerifier: %v", err)
	}
	
	// Parse the result
	if len(result) == 0 {
		return false, nil
	}
	
	return result[len(result)-1] == 1, nil
}

// GetAssignedFarms returns the list of farms assigned to the verifier
func (c *Client) GetAssignedFarms() ([]int64, error) {
	// Check if the verifier is registered first
	isRegistered, err := c.IsRegisteredVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to check if verifier is registered: %v", err)
	}
	
	if !isRegistered {
		return []int64{}, nil
	}
	
	// For each farm ID (1-8), check if the verifier is registered for it
	var farms []int64
	for farmID := int64(1); farmID <= 8; farmID++ {
		isRegisteredForFarm, err := c.isVerifierRegisteredForFarm(farmID)
		if err != nil {
			continue
		}
		
		if isRegisteredForFarm {
			farms = append(farms, farmID)
		}
	}
	
	return farms, nil
}

// isVerifierRegisteredForFarm checks if the verifier is registered for a specific farm
func (c *Client) isVerifierRegisteredForFarm(farmID int64) (bool, error) {
	// Create the method signature for isVerifierRegisteredForFarm
	methodSig := []byte("isVerifierRegisteredForFarm(uint256,address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedAddress := common.LeftPadBytes(c.address.Bytes(), 32)
	
	// Create the call data
	data := append(methodID, append(paddedFarmID, paddedAddress...)...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return false, fmt.Errorf("failed to call isVerifierRegisteredForFarm: %v", err)
	}
	
	// Parse the result
	if len(result) == 0 {
		return false, nil
	}
	
	return result[len(result)-1] == 1, nil
}

// IsVerifierActiveForFarm checks if the verifier is active for a farm
func (c *Client) IsVerifierActiveForFarm(farmID int64) (bool, error) {
	// Create the method signature for isVerifierActiveForFarm
	methodSig := []byte("isVerifierActiveForFarm(uint256,address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedAddress := common.LeftPadBytes(c.GetAddress().Bytes(), 32)
	
	// Create the call data
	data := append(methodID, append(paddedFarmID, paddedAddress...)...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return false, fmt.Errorf("failed to call isVerifierActiveForFarm: %v", err)
	}
	
	// Parse the result
	if len(result) == 0 {
		return false, nil
	}
	
	return result[len(result)-1] == 1, nil
}

// GetVerifierMetrics returns the metrics of the verifier
func (c *Client) GetVerifierMetrics() (*VerifierMetrics, error) {
	// Create the method signature for getVerifierMetrics
	methodSig := []byte("getVerifierMetrics(address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedAddress := common.LeftPadBytes(c.GetAddress().Bytes(), 32)
	
	// Create the call data
	data := append(methodID, paddedAddress...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call getVerifierMetrics: %v", err)
	}
	
	// Parse the result
	if len(result) < 160 { // 5 uint256 values (32 bytes each)
		return nil, fmt.Errorf("invalid result length: %d", len(result))
	}
	
	// Parse each uint256 value from the result
	verificationsPerformed := new(big.Int).SetBytes(result[0:32])
	lastActiveTimestamp := new(big.Int).SetBytes(result[32:64])
	totalUptime := new(big.Int).SetBytes(result[64:96])
	accumulatedRewards := new(big.Int).SetBytes(result[96:128])
	lastRewardsClaim := new(big.Int).SetBytes(result[128:160])
	
	return &VerifierMetrics{
		VerificationsPerformed: int64(verificationsPerformed.Uint64()),
		LastActiveTimestamp:    lastActiveTimestamp.Uint64(),
		TotalUptime:            totalUptime.Uint64(),
		AccumulatedRewards:     accumulatedRewards,
		LastRewardsClaim:       lastRewardsClaim.Uint64(),
	}, nil
}

// CalculatePendingRewards calculates the pending rewards for the verifier
func (c *Client) CalculatePendingRewards() (*big.Int, error) {
	// Create the method signature for calculatePendingRewards
	methodSig := []byte("calculatePendingRewards(address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedAddress := common.LeftPadBytes(c.GetAddress().Bytes(), 32)
	
	// Create the call data
	data := append(methodID, paddedAddress...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call calculatePendingRewards: %v", err)
	}
	
	// Parse the result
	if len(result) < 32 {
		return nil, fmt.Errorf("invalid result length: %d", len(result))
	}
	
	// Parse the uint256 value from the result
	pendingRewards := new(big.Int).SetBytes(result[0:32])
	
	return pendingRewards, nil
}

// GetClaimedRewards returns the total claimed rewards for the verifier
func (c *Client) GetClaimedRewards() (*big.Int, error) {
	// In a real implementation, we would call the contract to get the claimed rewards
	// For now, return a mock implementation
	return big.NewInt(5000000000000000000), nil // 5 DXP
}

// ClaimRewards claims the accumulated rewards for the verifier
func (c *Client) ClaimRewards() (*types.Transaction, error) {
	// Create the method signature for claimRewards
	methodSig := []byte("claimRewards()")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Create the transaction data
	data := methodID
	
	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.contractAddress, big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("failed to create and sign transaction: %v", err)
	}
	
	// Send the transaction
	err = c.ethClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	
	return tx, nil
}

// SubmitVerification submits a verification for a farm
func (c *Client) SubmitVerification(farmID int64, score *big.Float) (*types.Transaction, error) {
	// Convert score to uint256
	scoreInt := new(big.Int)
	score.Mul(score, big.NewFloat(100)).Int(scoreInt) // Convert to percentage * 100
	
	// Create the method signature for submitVerification
	methodSig := []byte("submitVerification(uint8,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedScore := common.LeftPadBytes(scoreInt.Bytes(), 32)
	
	// Create the transaction data
	data := append(methodID, append(paddedFarmID, paddedScore...)...)
	
	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.contractAddress, big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("failed to create and sign transaction: %v", err)
	}
	
	// Send the transaction
	err = c.ethClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	
	return tx, nil
}

// WithdrawVerifierStake withdraws the verifier stake
func (c *Client) WithdrawVerifierStake(amount *big.Int) (*types.Transaction, error) {
	// Create the method signature for withdrawVerifierStake
	methodSig := []byte("withdrawVerifierStake(address,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	address := c.GetAddress()
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)
	
	// Create the transaction data
	data := append(methodID, append(paddedAddress, paddedAmount...)...)
	
	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.contractAddress, big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("failed to create and sign transaction: %v", err)
	}
	
	// Send the transaction
	err = c.ethClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	
	return tx, nil
}

// WaitForTransaction waits for a transaction to be mined
func (c *Client) WaitForTransaction(txHash string) (*types.Receipt, error) {
	// Parse the transaction hash
	hash := common.HexToHash(txHash)
	
	// Wait for the transaction to be mined
	for {
		// Check if the transaction has been mined
		receipt, err := c.GetTransactionReceipt(hash)
		if err == nil {
			return receipt, nil
		}
		
		// Check if the error is "not found"
		if err == ethereum.NotFound {
			// Wait for a short time before checking again
			time.Sleep(2 * time.Second)
			continue
		}
		
		// Return other errors
		return nil, fmt.Errorf("failed to get transaction receipt: %v", err)
	}
}

// GetWalletAddress returns the wallet address as a string
func (c *Client) GetWalletAddress() string {
	return c.GetAddress().Hex()
}

// GetContractAddress returns the contract address as a string
func (c *Client) GetContractAddress() string {
	return c.contractAddress.Hex()
}

// GetETHBalance returns the ETH balance of the client
func (c *Client) GetETHBalance() (*big.Int, error) {
	return c.ethClient.BalanceAt(context.Background(), c.GetAddress(), nil)
}

// GetCurrentBlock returns the current block number
func (c *Client) GetCurrentBlock() (uint64, error) {
	return c.ethClient.BlockNumber(context.Background())
}

// GetDXPAllowance returns the DXP token allowance for the contract
func (c *Client) GetDXPAllowance(owner, spender common.Address) (*big.Int, error) {
	// Create the method signature for allowance
	methodSig := []byte("allowance(address,address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedOwner := common.LeftPadBytes(owner.Bytes(), 32)
	paddedSpender := common.LeftPadBytes(spender.Bytes(), 32)
	
	// Create the call data
	data := append(methodID, append(paddedOwner, paddedSpender...)...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.tokenAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call allowance: %v", err)
	}
	
	// Parse the result
	allowance := new(big.Int).SetBytes(result)
	return allowance, nil
}

// CheckVerifierStatus checks if the current wallet is registered as a verifier
func (c *Client) CheckVerifierStatus() (bool, error) {
	return c.IsRegisteredVerifier()
}

// GetVerifierStake returns the stake amount of the verifier
func (c *Client) GetVerifierStake() (*big.Int, error) {
	// Create the method signature for getVerifierStake
	methodSig := []byte("getVerifierStake(address)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the address parameter
	address := c.GetAddress()
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	
	// Create the call data
	data := append(methodID, paddedAddress...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call getVerifierStake: %v", err)
	}
	
	// Parse the result
	if len(result) == 0 {
		return big.NewInt(0), nil
	}
	
	stake := new(big.Int).SetBytes(result)
	return stake, nil
}

// GetFarmData returns data about a farm
func (c *Client) GetFarmData(farmID int64) (*FarmData, error) {
	// Create the method signature for getFarmData
	methodSig := []byte("getFarmData(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the farmID parameter
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	
	// Create the call data
	data := append(methodID, paddedFarmID...)
	
	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}
	
	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call getFarmData: %v", err)
	}
	
	// Parse the result (3 uint256 values)
	if len(result) < 96 {
		return nil, fmt.Errorf("invalid result length")
	}
	
	score := new(big.Int).SetBytes(result[0:32]).Uint64()
	benchmark := new(big.Int).SetBytes(result[32:64]).Uint64()
	lastUpdate := new(big.Int).SetBytes(result[64:96]).Uint64()
	
	return &FarmData{
		Score:      score,
		Benchmark:  benchmark,
		LastUpdate: lastUpdate,
	}, nil
}

// SetFarmBenchmark sets the benchmark for a farm
func (c *Client) SetFarmBenchmark(farmID int64, benchmark *big.Int) (*types.Transaction, error) {
	// Create the method signature for setFarmBenchmark
	methodSig := []byte("setFarmBenchmark(uint256,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedBenchmark := common.LeftPadBytes(benchmark.Bytes(), 32)
	
	// Create the call data
	data := append(methodID, append(paddedFarmID, paddedBenchmark...)...)
	
	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.contractAddress, big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %v", err)
	}
	
	// Send the transaction
	err = c.ethClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	
	return tx, nil
}

// SubmitFarmScore submits a score for a farm
func (c *Client) SubmitFarmScore(farmID int64, score *big.Int) (*types.Transaction, error) {
	// Create the method signature for submitFarmScore
	methodSig := []byte("submitFarmScore(uint256,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]
	
	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedScore := common.LeftPadBytes(score.Bytes(), 32)
	
	// Create the call data
	data := append(methodID, append(paddedFarmID, paddedScore...)...)
	
	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.contractAddress, big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %v", err)
	}
	
	// Send the transaction
	err = c.ethClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	
	return tx, nil
}

// GetTransactionReceipt gets the receipt of a transaction by its hash
func (c *Client) GetTransactionReceipt(txHash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	return c.ethClient.TransactionReceipt(ctx, txHash)
}

// Helper function to create and sign a transaction
func (c *Client) createAndSignTransaction(to common.Address, value *big.Int, data []byte) (*types.Transaction, error) {
	// Get the current nonce
	nonce, err := c.ethClient.PendingNonceAt(context.Background(), c.GetAddress())
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %v", err)
	}
	
	// Get gas price
	gasPrice, err := c.ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %v", err)
	}
	
	// Apply gas price multiplier from environment variable (default to 2.0 if not set)
	multiplier := 2.0
	if multiplierStr := os.Getenv("GAS_PRICE_MULTIPLIER"); multiplierStr != "" {
		if m, err := strconv.ParseFloat(multiplierStr, 64); err == nil {
			multiplier = m
		}
	}
	
	// Increase gas price by multiplier
	adjustedGasPrice := new(big.Int).Mul(gasPrice, big.NewInt(int64(multiplier*100)))
	adjustedGasPrice = new(big.Int).Div(adjustedGasPrice, big.NewInt(100))
	
	// Get gas limit from environment variable (default to 500000 if not set)
	gasLimit := uint64(500000)
	if gasLimitStr := os.Getenv("GAS_LIMIT"); gasLimitStr != "" {
		if gl, err := strconv.ParseUint(gasLimitStr, 10, 64); err == nil {
			gasLimit = gl
		}
	}
	
	// Create the transaction
	tx := types.NewTransaction(nonce, to, value, gasLimit, adjustedGasPrice, data)
	
	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(c.chainID), c.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}
	
	return signedTx, nil
}
