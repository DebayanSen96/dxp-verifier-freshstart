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


// FarmData represents data about a farm
type FarmData struct {
	Score      uint64
	Benchmark  uint64
	LastUpdate uint64
}

// Client represents an Ethereum client
type Client struct {
	ethClient         *ethclient.Client
	privateKey        *ecdsa.PrivateKey
	protocolAddress   common.Address
	consensusAddress  common.Address
	tokenAddress      common.Address
	chainID           *big.Int
	address           common.Address
}

// NewClient creates a new Ethereum client
func NewClient(rpcURL, privateKeyHex, protocolAddress, consensusAddress, tokenAddress string) (*Client, error) {
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
	//get chain ID from the contract address as mentioned in the protocol core blah blah
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
		ethClient:        ethClient,
		privateKey:       privateKey,
		protocolAddress:  common.HexToAddress(protocolAddress),
		consensusAddress: common.HexToAddress(consensusAddress),
		tokenAddress:     common.HexToAddress(tokenAddress),
		chainID:          chainID,
		address:          crypto.PubkeyToAddress(privateKey.PublicKey),
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

	// Pack the address parameter
	paddedAddress := common.LeftPadBytes(c.protocolAddress.Bytes(), 32)
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
	// Use the correct method signature for ProtocolCore: registerAsVerifier(uint256,uint256)
	methodSig := []byte("registerAsVerifier(uint256,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters: farmId, amount
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedStakeAmount := common.LeftPadBytes(stakeAmount.Bytes(), 32)

	// Create the transaction data
	data := append(methodID, paddedFarmID...)
	data = append(data, paddedStakeAmount...)

	fmt.Printf("[DEBUG] Calling registerAsVerifier at %s with farmId=%d, amount=%s\n", c.protocolAddress.Hex(), farmID, stakeAmount.String())
	fmt.Printf("[DEBUG] Call data: %x\n", data)

	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.protocolAddress, big.NewInt(0), data)
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

// IsRegisteredVerifier checks if the current wallet is an approved verifier for a farm (default farmId = 1)
func (c *Client) IsRegisteredVerifier() (bool, error) {
	// Use farmId 1 as default; update if you want to check a different farm
	farmId := big.NewInt(1)
	address := c.address

	// Create the method signature for isApprovedVerifier(uint256,address)
	methodSig := []byte("isApprovedVerifier(uint256,address)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmId := common.LeftPadBytes(farmId.Bytes(), 32)
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmId...)
	data = append(data, paddedAddress...)

	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}

	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return false, fmt.Errorf("failed to call isApprovedVerifier: %v", err)
	}

	if len(result) == 0 {
		return false, nil
	}

	// Solidity returns bool as 32 bytes, last byte is 1 for true, 0 for false
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
	// Create the method signature for isApprovedVerifier
	methodSig := []byte("isApprovedVerifier(uint256,address)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedAddress := common.LeftPadBytes(c.address.Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)
	data = append(data, paddedAddress...)

	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}

	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return false, fmt.Errorf("failed to call isApprovedVerifier: %v", err)
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
	data := append(methodID, paddedFarmID...)
	data = append(data, paddedAddress...)

	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.protocolAddress,
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

// CalculatePendingRewards calculates the pending rewards for the verifier
func (c *Client) CalculatePendingRewards() (*big.Int, error) {
	// Create the method signature for calculatePendingRewards
	methodSig := []byte("calculatePendingRewards(address)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the address parameter
	address := c.GetAddress()
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedAddress...)

	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.protocolAddress,
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
	tx, err := c.createAndSignTransaction(c.protocolAddress, big.NewInt(0), data)
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
	// Create the method signature for submit in Consensus contract
	methodSig := []byte("submit(uint256,uint256,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Convert score to uint256 (normalized to 1e18 scale)
	scoreInt := new(big.Int)
	score.Mul(score, big.NewFloat(1e18)).Int(scoreInt) // Convert to 1e18 scale (0.5 = 0.5 * 10^18)

	// Calculate benchmark (for now we'll use the same value for both)
	benchmarkInt := new(big.Int).Set(scoreInt)

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedScore := common.LeftPadBytes(scoreInt.Bytes(), 32)
	paddedBenchmark := common.LeftPadBytes(benchmarkInt.Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)
	data = append(data, paddedScore...)
	data = append(data, paddedBenchmark...)

	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.consensusAddress, big.NewInt(0), data)
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
func (c *Client) WithdrawVerifierStake(farmID int64, amount *big.Int) (*types.Transaction, error) {
	// Create the method signature for withdrawVerifierStake
	methodSig := []byte("withdrawVerifierStake(uint256,uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	farmIdBig := big.NewInt(farmID)
	paddedFarmId := common.LeftPadBytes(farmIdBig.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmId...)
	data = append(data, paddedAmount...)

	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.protocolAddress, big.NewInt(0), data)
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

// GetProtocolAddress returns the protocol contract address as a string
func (c *Client) GetProtocolAddress() string {
	return c.protocolAddress.Hex()
}

// GetConsensusAddress returns the consensus contract address as a string
func (c *Client) GetConsensusAddress() string {
	return c.consensusAddress.Hex()
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

// GetVerifierStake returns the stake amount of the verifier for a specific farm
func (c *Client) GetVerifierStake(farmID int64) (*big.Int, error) {
	// Use the correct method signature for the public mapping: verifierStakes(uint256,address)
	methodSig := []byte("verifierStakes(uint256,address)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters: farmId, address
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	address := c.GetAddress()
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)
	data = append(data, paddedAddress...)

	// Create the call message
	msg := ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}

	// Call the contract
	result, err := c.ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call verifierStakes: %v", err)
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
		To:   &c.protocolAddress,
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

// SetFarmBenchmark sets the benchmark for a farm (legacy method, use SetFarmBenchmarkSecure instead)
func (c *Client) SetFarmBenchmark(farmID int64, benchmark *big.Int) (*types.Transaction, error) {
	// This function is kept for backward compatibility
	// It's recommended to use SetFarmBenchmarkSecure instead which includes ECDSA verification
	return c.SetFarmBenchmarkSecure(farmID, benchmark)
}

// SetFarmBenchmarkSecure sets the benchmark for a farm with ECDSA verification
func (c *Client) SetFarmBenchmarkSecure(farmID int64, benchmark *big.Int) (*types.Transaction, error) {
	// Check if we are the current leader - we should be since we just submitted the score
	leader, err := c.GetFarmLeader(farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to get farm leader: %v", err)
	}

	if leader != c.address {
		return nil, fmt.Errorf("cannot set benchmark: farm %d has a different leader: %s (we are %s)", farmID, leader.Hex(), c.address.Hex())
	}

	// We're the leader, proceed with benchmark submission

	// Create a message hash from the benchmark data for signing
	// This must match the contract's implementation: keccak256(abi.encodePacked(farmId, benchmark, msg.sender))
	packedData := append(common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32),
		append(common.LeftPadBytes(benchmark.Bytes(), 32),
			c.address.Bytes()...)...)
	messageHash := crypto.Keccak256Hash(packedData)

	// Convert to Ethereum signed message hash (same as MessageHashUtils.toEthSignedMessageHash in Solidity)
	// This prefixes the hash with "\x19Ethereum Signed Message:\n32" before hashing again
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	dataToSign := append(prefix, messageHash.Bytes()...)
	ethSignedMessageHash := crypto.Keccak256Hash(dataToSign)

	// Sign the Ethereum signed message hash with the private key
	signature, err := crypto.Sign(ethSignedMessageHash.Bytes(), c.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign benchmark data: %v", err)
	}

	// Adjust v value for Ethereum (Solidity expects 27/28, Go returns 0/1)
	if len(signature) == 65 && (signature[64] == 0 || signature[64] == 1) {
		signature[64] += 27
	}

	// Create the method signature for setFarmBenchmarkSecure
	methodSig := []byte("setFarmBenchmarkSecure(uint256,uint256,bytes)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedBenchmark := common.LeftPadBytes(benchmark.Bytes(), 32)

	// For the signature, we need to encode it as a bytes array
	// First, we need to encode the offset to the signature data
	sigOffset := common.LeftPadBytes(big.NewInt(96).Bytes(), 32) // 96 bytes offset (2 previous params * 32 bytes)

	// Then we encode the length of the signature
	sigLength := common.LeftPadBytes(big.NewInt(int64(len(signature))).Bytes(), 32)

	// Pad the signature to a multiple of 32 bytes
	padLen := (32 - len(signature)%32) % 32
	paddedSig := append(signature, make([]byte, padLen)...)

	// Create the call data
	data := append(methodID, append(paddedFarmID, append(paddedBenchmark, append(sigOffset, append(sigLength, paddedSig...)...)...)...)...)

	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.protocolAddress, big.NewInt(0), data)
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

// SubmitFarmScore submits a score for a farm with ECDSA verification (only callable by the current farm leader)
func (c *Client) SubmitFarmScore(farmID int64, score *big.Int) (*types.Transaction, error) {
	// First check if we're already the leader or can register as the leader
	hasLeader, err := c.CheckFarmLeader(farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to check farm leader status: %v", err)
	}

	if hasLeader {
		// Check if we are the current leader
		leader, err := c.GetFarmLeader(farmID)
		if err != nil {
			return nil, fmt.Errorf("failed to get farm leader: %v", err)
		}

		if leader != c.address {
			return nil, fmt.Errorf("cannot submit score: farm %d already has another leader: %s", farmID, leader.Hex())
		}

		// We're already the leader, proceed with score submission without additional logging
	} else {
		// Try to register as the farm leader
		tx, err := c.RegisterFarmLeader(farmID)
		if err != nil {
			return nil, fmt.Errorf("failed to register as farm leader: %v", err)
		}

		if tx != nil {
			// Wait for the leader registration transaction to be mined
			fmt.Printf("Waiting for leader registration transaction to be mined...\n")
			time.Sleep(5 * time.Second)
		}
	}

	// Create a message hash from the score data for signing
	// This must match the contract's implementation: keccak256(abi.encodePacked(farmId, score, msg.sender))
	packedData := append(common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32),
		append(common.LeftPadBytes(score.Bytes(), 32),
			c.address.Bytes()...)...)
	messageHash := crypto.Keccak256Hash(packedData)

	// Convert to Ethereum signed message hash (same as MessageHashUtils.toEthSignedMessageHash in Solidity)
	// This prefixes the hash with "\x19Ethereum Signed Message:\n32" before hashing again
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	dataToSign := append(prefix, messageHash.Bytes()...)
	ethSignedMessageHash := crypto.Keccak256Hash(dataToSign)

	// Sign the Ethereum signed message hash with the private key
	signature, err := crypto.Sign(ethSignedMessageHash.Bytes(), c.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign score data: %v", err)
	}

	// Adjust v value for Ethereum (Solidity expects 27/28, Go returns 0/1)
	if len(signature) == 65 && (signature[64] == 0 || signature[64] == 1) {
		signature[64] += 27
	}

	// Create the method signature for submitFarmScore
	methodSig := []byte("submitFarmScore(uint256,uint256,bytes)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)
	paddedScore := common.LeftPadBytes(score.Bytes(), 32)

	// For the signature, we need to encode it as a bytes array
	// First, we need to encode the offset to the signature data
	sigOffset := common.LeftPadBytes(big.NewInt(96).Bytes(), 32) // 96 bytes offset (2 previous params * 32 bytes)

	// Then we encode the length of the signature
	sigLength := common.LeftPadBytes(big.NewInt(int64(len(signature))).Bytes(), 32)

	// Pad the signature to a multiple of 32 bytes
	padLen := (32 - len(signature)%32) % 32
	paddedSig := append(signature, make([]byte, padLen)...)

	// Create the call data
	data := append(methodID, append(paddedFarmID, append(paddedScore, append(sigOffset, append(sigLength, paddedSig...)...)...)...)...)

	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.protocolAddress, big.NewInt(0), data)
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

// CheckFarmLeader checks if there's already a leader for a farm
func (c *Client) CheckFarmLeader(farmID int64) (bool, error) {
	// Create the method signature for hasFarmLeader
	methodSig := []byte("hasFarmLeader(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

	// Call the contract
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check if farm has leader: %v", err)
	}

	// Parse the result (boolean)
	if len(result) < 32 {
		return false, fmt.Errorf("invalid result length")
	}

	// Check if the result is true (has leader)
	hasLeader := new(big.Int).SetBytes(result).Uint64() > 0
	return hasLeader, nil
}

// GetFarmLeader gets the current leader for a farm
func (c *Client) GetFarmLeader(farmID int64) (common.Address, error) {
	// Create the method signature for getFarmLeader
	methodSig := []byte("getFarmLeader(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

	// Call the contract
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to get farm leader: %v", err)
	}

	// Parse the result (address)
	if len(result) < 32 {
		return common.Address{}, fmt.Errorf("invalid result length")
	}

	// Extract the address from the result
	var addr common.Address
	copy(addr[:], result[12:32]) // Addresses are 20 bytes, padded to 32 bytes
	return addr, nil
}

// GetCurrentFarmRound gets the current consensus round number for a farm
func (c *Client) GetCurrentFarmRound(farmID int64) (uint64, error) {
	// Create the method signature for farmConsensusRound
	methodSig := []byte("farmConsensusRound(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

	// Call the contract
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get farm consensus round: %v", err)
	}

	// Parse the result (uint256)
	if len(result) < 32 {
		return 0, fmt.Errorf("invalid result length")
	}

	// Convert the result to uint64
	roundNumber := new(big.Int).SetBytes(result).Uint64()
	return roundNumber, nil
}

// RegisterFarmLeader registers the caller as the leader for a farm consensus round
func (c *Client) RegisterFarmLeader(farmID int64) (*types.Transaction, error) {
	// Check if the caller is registered for this farm
	isRegistered, err := c.IsVerifierActiveForFarm(farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if verifier is registered for farm: %v", err)
	}
	if !isRegistered {
		return nil, fmt.Errorf("verifier is not registered for farm %d", farmID)
	}

	// Check if there's already a leader for this farm
	hasLeader, err := c.CheckFarmLeader(farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if farm has leader: %v", err)
	}

	if hasLeader {
		// Check if we are the current leader
		leader, err := c.GetFarmLeader(farmID)
		if err != nil {
			return nil, fmt.Errorf("failed to get farm leader: %v", err)
		}

		if leader == c.address {
			// We're already the leader, but we still need to call the contract to increment the round
			// Continue with the function to make the contract call
		} else {
			// Someone else is the leader
			return nil, fmt.Errorf("farm %d already has an active leader: %s", farmID, leader.Hex())
		}
	}

	// Create the method signature for registerFarmLeader
	methodSig := []byte("registerFarmLeader(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

	// Create and sign the transaction
	tx, err := c.createAndSignTransaction(c.protocolAddress, big.NewInt(0), data)
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

// GetFarmScore gets the current score for a farm from the blockchain
func (c *Client) GetFarmScore(farmId int64) (*big.Int, error) {
	// Create the method signature for farmScores(uint256)
	methodSig := []byte("farmScores(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmId).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

	// Call the contract
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get farm score: %v", err)
	}

	// Parse the result (uint256)
	if len(result) < 32 {
		return nil, fmt.Errorf("invalid result length")
	}

	// Return the score as a big.Int
	return new(big.Int).SetBytes(result), nil
}

// GetFarmBenchmark gets the current benchmark for a farm from the blockchain
func (c *Client) GetFarmBenchmark(farmId int64) (*big.Int, error) {
	// Create the method signature for farmBenchmarks(uint256)
	methodSig := []byte("farmBenchmarks(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the parameters
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmId).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

	// Call the contract
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &c.protocolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get farm benchmark: %v", err)
	}

	// Parse the result (uint256)
	if len(result) < 32 {
		return nil, fmt.Errorf("invalid result length")
	}

	// Return the benchmark as a big.Int
	return new(big.Int).SetBytes(result), nil
}

// SubmitConsensusResult submits consensus results for a farm to the blockchain
func (c *Client) SubmitConsensusResult(farmId int64, score float64, participants []string) (string, error) {
	// First check if we're already the leader or can register as the leader
	hasLeader, err := c.CheckFarmLeader(farmId)
	if err != nil {
		return "", fmt.Errorf("failed to check farm leader status: %v", err)
	}

	if hasLeader {
		// Check if we are the current leader
		leader, err := c.GetFarmLeader(farmId)
		if err != nil {
			return "", fmt.Errorf("failed to get farm leader: %v", err)
		}

		if leader != c.address {
			return "", fmt.Errorf("cannot submit consensus result: farm %d already has another leader: %s", farmId, leader.Hex())
		}

		// We're already the leader, proceed with score submission without additional logging
	} else {
		// Try to register as the farm leader
		tx, err := c.RegisterFarmLeader(farmId)
		if err != nil {
			return "", fmt.Errorf("failed to register as farm leader: %v", err)
		}

		if tx != nil {
			// Wait for the leader registration transaction to be mined
			fmt.Printf("Waiting for leader registration transaction to be mined...\n")
			time.Sleep(5 * time.Second)
		}
	}

	// Convert score to uint256 (normalized to 1e18 scale)
	scoreInt := new(big.Int)
	scoreFloat := big.NewFloat(score)
	scoreFloat.Mul(scoreFloat, big.NewFloat(1e18)).Int(scoreInt) // Convert to 1e18 scale (0.5 = 0.5 * 10^18)

	// Submit the farm score as the leader
	tx, err := c.SubmitFarmScore(farmId, scoreInt)
	if err != nil {
		return "", fmt.Errorf("failed to submit consensus result: %v", err)
	}

	// Return the transaction hash
	return tx.Hash().Hex(), nil
}

// GetTransactionStatus checks if a transaction has been mined
func (c *Client) GetTransactionStatus(txHash string) (map[string]interface{}, error) {
	// Parse the transaction hash
	hash := common.HexToHash(txHash)

	// Check if the transaction has been mined
	receipt, err := c.GetTransactionReceipt(hash)
	if err != nil {
		if err == ethereum.NotFound {
			// Transaction not yet mined
			return map[string]interface{}{
				"mined":  false,
				"status": "pending",
			}, nil
		}
		return nil, fmt.Errorf("failed to get transaction receipt: %v", err)
	}

	// Transaction has been mined
	status := "success"
	if receipt.Status == 0 {
		status = "failed"
	}

	return map[string]interface{}{
		"mined":       true,
		"status":      status,
		"blockNumber": receipt.BlockNumber.Uint64(),
		"blockHash":   receipt.BlockHash.Hex(),
		"gasUsed":     receipt.GasUsed,
	}, nil
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
