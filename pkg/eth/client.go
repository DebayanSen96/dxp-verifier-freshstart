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
	ethClient       *ethclient.Client
	contractABI     abi.ABI
	contractBound   *bind.BoundContract
	tokenABI        abi.ABI
	tokenBound      *bind.BoundContract
	walletAddress   common.Address
	contractAddress common.Address
	tokenAddress    common.Address
	privateKey      *ecdsa.PrivateKey
}

// NewClient creates a new Ethereum client
func NewClient() (*Client, error) {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Warning: Error loading .env file: %v\n", err)
	}

	// Load RPC URL from .env
	rpcURL := os.Getenv("BASE_RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("BASE_RPC_URL not set in .env file")
	}

	// Load contract address from .env
	contractAddress := os.Getenv("DXP_CONTRACT_ADDRESS")
	if contractAddress == "" {
		return nil, fmt.Errorf("DXP_CONTRACT_ADDRESS not set in .env file")
	}

	// Load private key from .env
	privateKeyHex := os.Getenv("WALLET_PRIVATE_KEY")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("WALLET_PRIVATE_KEY not set in .env file")
	}

	// Add 0x prefix if not present
	if !strings.HasPrefix(privateKeyHex, "0x") {
		privateKeyHex = "0x" + privateKeyHex
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Get wallet address from private key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast public key to ECDSA")
	}
	walletAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// Connect to Ethereum client
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %v", err)
	}

	// Parse contract ABI
	const DexponentProtocolABI = `[{"inputs":[{"internalType":"address","name":"verifierAddress","type":"address"},{"internalType":"uint256","name":"stakeAmount","type":"uint256"}],"name":"registerVerifier","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"farmId","type":"uint256"},{"internalType":"uint256","name":"score","type":"uint256"},{"internalType":"address[]","name":"participants","type":"address[]"}],"name":"submitVerification","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"verifierStake","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"verifierAddress","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"withdrawVerifierStake","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"MIN_VERIFIER_STAKE","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"verifier","type":"address"}],"name":"isVerifier","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"verifier","type":"address"}],"name":"getVerifierStake","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"verifierAddress","type":"address"}],"name":"checkAndUpdateVerifierStatus","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"verifierAddress","type":"address"},{"internalType":"uint256","name":"additionalStake","type":"uint256"}],"name":"increaseVerifierStake","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
	parsedContractABI, err := abi.JSON(strings.NewReader(DexponentProtocolABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %v", err)
	}

	// Create contract instance
	contractAddr := common.HexToAddress(contractAddress)

	// Create bound contract
	contractBound := bind.NewBoundContract(contractAddr, parsedContractABI, ethClient, ethClient, ethClient)

	// DXP token address from .env
	dxpTokenAddress := os.Getenv("DXP_TOKEN_ADDRESS")
	if dxpTokenAddress == "" {
		return nil, fmt.Errorf("DXP_TOKEN_ADDRESS not set in .env file")
	}

	// Create token contract instance
	tokenAddress := common.HexToAddress(dxpTokenAddress)

	// Parse token ABI
	const DXPTokenABI = `[{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]`
	tokenABI, err := abi.JSON(strings.NewReader(DXPTokenABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ABI: %v", err)
	}

	tokenBound := bind.NewBoundContract(tokenAddress, tokenABI, ethClient, ethClient, ethClient)

	return &Client{
		ethClient:       ethClient,
		contractABI:     parsedContractABI,
		contractBound:   contractBound,
		tokenABI:        tokenABI,
		tokenBound:      tokenBound,
		walletAddress:   walletAddress,
		contractAddress: contractAddr,
		tokenAddress:    tokenAddress,
		privateKey:      privateKey,
	}, nil
}

// RegisterVerifier registers the current wallet as a verifier
func (c *Client) RegisterVerifier(amount *big.Int) (string, error) {
	fmt.Printf("Registering %s as a verifier with %s tokens...\n", c.walletAddress.Hex(), formatTokenAmount(amount))

	// Check if already registered
	status, err := c.CheckVerifierStatus()
	if err != nil {
		return "", fmt.Errorf("failed to check verifier status: %v", err)
	}

	// If already registered, check if we need to increase stake
	if registered, ok := status["registered"].(bool); ok && registered {
		// If stake is sufficient, just return
		if stakeOK, ok := status["stakeOK"].(bool); ok && stakeOK {
			fmt.Println("Already registered as a verifier with sufficient stake")
			return "", nil
		}

		// If stake is insufficient, increase it
		currentStake, ok1 := status["stake"].(*big.Int)
		minStake, ok2 := status["minStake"].(*big.Int)
		
		if ok1 && ok2 {
			neededStake := new(big.Int).Sub(minStake, currentStake)
			
			if amount.Cmp(neededStake) < 0 {
				return "", fmt.Errorf("insufficient additional stake. You need at least %s more DXP to meet the minimum requirement", formatTokenAmount(neededStake))
			}
			
			fmt.Printf("Already registered but stake is insufficient. Increasing stake by %s DXP...\n", formatTokenAmount(amount))
			return c.IncreaseVerifierStake(amount)
		}
	}

	// Check DXP balance
	balance, err := c.GetDXPBalance(c.walletAddress.Hex())
	if err != nil {
		return "", fmt.Errorf("failed to get DXP balance: %v", err)
	}
	if balance.Cmp(amount) < 0 {
		return "", fmt.Errorf("insufficient DXP balance: have %s, need %s",
			formatTokenAmount(balance), formatTokenAmount(amount))
	}

	// Check minimum stake
	minStake, err := c.GetMinVerifierStake()
	if err != nil {
		return "", fmt.Errorf("failed to get minimum stake: %v", err)
	}
	if amount.Cmp(minStake) < 0 {
		return "", fmt.Errorf("stake amount (%s) is below minimum requirement (%s)",
			formatTokenAmount(amount), formatTokenAmount(minStake))
	}

	// Check allowance
	allowance, err := c.GetDXPAllowance(c.walletAddress.Hex(), c.contractAddress.Hex())
	if err != nil {
		return "", fmt.Errorf("failed to get DXP allowance: %v", err)
	}
	if allowance.Cmp(amount) < 0 {
		// Approve DXP tokens
		fmt.Println("Approving DXP tokens...")
		approvalTx, err := c.ApproveDXPToken(amount)
		if err != nil {
			return "", fmt.Errorf("failed to approve DXP tokens: %v", err)
		}
		fmt.Printf("Approval transaction sent: %s\n", approvalTx)
		fmt.Println("Waiting for approval confirmation...")
		_, err = c.WaitForTransaction(approvalTx)
		if err != nil {
			return "", fmt.Errorf("approval transaction failed: %v", err)
		}
	}

	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return "", fmt.Errorf("failed to create transaction options: %v", err)
	}

	// Create transaction data for registerVerifier function
	data, err := c.contractABI.Pack(
		"registerVerifier",
		c.walletAddress, // verifierAddress
		amount,          // stakeAmount
	)
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %w", err)
	}

	// Send transaction
	tx, err := c.sendTransaction(auth, data)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return tx.Hash().Hex(), nil
}

// IncreaseVerifierStake increases the stake for the current wallet as a verifier
func (c *Client) IncreaseVerifierStake(amount *big.Int) (string, error) {
	fmt.Printf("Increasing verifier stake by %s tokens...\n", formatTokenAmount(amount))

	// Check DXP balance
	balance, err := c.GetDXPBalance(c.walletAddress.Hex())
	if err != nil {
		return "", fmt.Errorf("failed to get DXP balance: %v", err)
	}
	if balance.Cmp(amount) < 0 {
		return "", fmt.Errorf("insufficient DXP balance: have %s, need %s",
			formatTokenAmount(balance), formatTokenAmount(amount))
	}

	// Check allowance
	allowance, err := c.GetDXPAllowance(c.walletAddress.Hex(), c.contractAddress.Hex())
	if err != nil {
		return "", fmt.Errorf("failed to get DXP allowance: %v", err)
	}
	if allowance.Cmp(amount) < 0 {
		// Approve DXP tokens
		fmt.Println("Approving DXP tokens...")
		approvalTx, err := c.ApproveDXPToken(amount)
		if err != nil {
			return "", fmt.Errorf("failed to approve DXP tokens: %v", err)
		}
		fmt.Printf("Approval transaction sent: %s\n", approvalTx)
		fmt.Println("Waiting for approval confirmation...")
		_, err = c.WaitForTransaction(approvalTx)
		if err != nil {
			return "", fmt.Errorf("approval transaction failed: %v", err)
		}
	}

	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return "", fmt.Errorf("failed to create transaction options: %v", err)
	}

	// Create transaction data for increaseVerifierStake function
	data, err := c.contractABI.Pack(
		"increaseVerifierStake",
		c.walletAddress, // verifierAddress
		amount,          // additionalStake
	)
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %w", err)
	}

	// Send transaction
	tx, err := c.sendTransaction(auth, data)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	fmt.Printf("Stake increase transaction sent: %s\n", tx.Hash().Hex())

	return tx.Hash().Hex(), nil
}

// GetMinVerifierStake returns the minimum stake amount required for verifiers
func (c *Client) GetMinVerifierStake() (*big.Int, error) {
	// The contract has a constant MIN_VERIFIER_STAKE = 100
	// Convert to tokens with 18 decimals
	minStake := new(big.Int).Mul(
		big.NewInt(100),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)

	fmt.Printf("Minimum stake requirement: %s DXP\n", formatTokenAmount(minStake))
	return minStake, nil
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
		return "", fmt.Errorf("failed to pack transaction data: %w", err)
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

	// The new contract doesn't have rewards functionality
	// Return 0 to maintain compatibility with existing code
	fmt.Println("Note: The new contract doesn't support rewards. Returning 0.")
	return big.NewInt(0), nil
}

// ClaimRewards claims the pending rewards for the verifier
func (c *Client) ClaimRewards() (string, error) {
	// The new contract doesn't have rewards functionality
	return "", fmt.Errorf("the new contract doesn't support rewards functionality")
}

// IsRegisteredVerifier checks if the current wallet is registered as a verifier
func (c *Client) IsRegisteredVerifier() (bool, error) {
	fmt.Printf("Checking if %s is registered as a verifier...\n", c.walletAddress.Hex())

	// Create call options
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}

	// Call isVerifier function instead of registeredVerifiers mapping
	var result bool
	var out []interface{}
	err := c.contractBound.Call(callOpts, &out, "isVerifier", c.walletAddress)
	if err == nil && len(out) > 0 {
		result = out[0].(bool)
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if registered: %v", err)
	}

	return result, nil
}

// GetDXPBalance returns the DXP token balance of the wallet
func (c *Client) GetDXPBalance(address string) (*big.Int, error) {
	// DXP token address from .env
	dxpTokenAddress := os.Getenv("DXP_TOKEN_ADDRESS")
	if dxpTokenAddress == "" {
		dxpTokenAddress = "0x08Eb4d9c6e388777b21b517A13030a08e39AC279" // Default DXP token address on Sepolia
	}

	// Create token contract instance
	tokenAddress := common.HexToAddress(dxpTokenAddress)
	tokenABI := `[{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedTokenABI, err := abi.JSON(strings.NewReader(tokenABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ABI: %v", err)
	}

	tokenContract := bind.NewBoundContract(tokenAddress, parsedTokenABI, c.ethClient, c.ethClient, c.ethClient)

	// Call balanceOf function
	var out []interface{}
	callOpts := &bind.CallOpts{Context: context.Background()}
	err = tokenContract.Call(callOpts, &out, "balanceOf", common.HexToAddress(address))
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
func (c *Client) GetDXPAllowance(owner, spender string) (*big.Int, error) {
	// DXP token address from .env
	dxpTokenAddress := os.Getenv("DXP_TOKEN_ADDRESS")
	if dxpTokenAddress == "" {
		dxpTokenAddress = "0x08Eb4d9c6e388777b21b517A13030a08e39AC279" // Default DXP token address on Sepolia
	}

	// Create token contract instance
	tokenAddress := common.HexToAddress(dxpTokenAddress)
	tokenABI := `[{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedTokenABI, err := abi.JSON(strings.NewReader(tokenABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ABI: %v", err)
	}

	tokenContract := bind.NewBoundContract(tokenAddress, parsedTokenABI, c.ethClient, c.ethClient, c.ethClient)

	// Call allowance function
	var out []interface{}
	callOpts := &bind.CallOpts{Context: context.Background()}
	err = tokenContract.Call(callOpts, &out, "allowance", common.HexToAddress(owner), common.HexToAddress(spender))
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

// CheckVerifierStatus checks the status of the verifier and returns detailed information
func (c *Client) CheckVerifierStatus() (map[string]interface{}, error) {
	statusInfo := make(map[string]interface{})
	
	// Get wallet address
	statusInfo["address"] = c.walletAddress.Hex()
	
	// Check if registered using the contract's isVerifier function
	isRegistered, err := c.IsRegisteredVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to check if registered: %v", err)
	}
	statusInfo["registered"] = isRegistered
	
	// Get current stake
	stake, err := c.GetVerifierStake()
	if err != nil {
		return nil, fmt.Errorf("failed to get verifier stake: %v", err)
	}
	statusInfo["stake"] = stake
	
	// Get minimum stake
	minStake, err := c.GetMinVerifierStake()
	if err != nil {
		return nil, fmt.Errorf("failed to get minimum stake: %v", err)
	}
	statusInfo["minStake"] = minStake
	
	// Check if stake meets minimum requirement
	stakeOK := stake.Cmp(minStake) >= 0
	statusInfo["stakeOK"] = stakeOK
	
	// Get ETH balance
	ethBalance, err := c.GetETHBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH balance: %v", err)
	}
	statusInfo["ethBalance"] = ethBalance
	
	// Get DXP balance
	dxpBalance, err := c.GetDXPBalance(c.walletAddress.Hex())
	if err != nil {
		return nil, fmt.Errorf("failed to get DXP balance: %v", err)
	}
	statusInfo["dxpBalance"] = dxpBalance
	
	// Get DXP allowance
	dxpAllowance, err := c.GetDXPAllowance(c.walletAddress.Hex(), c.contractAddress.Hex())
	if err != nil {
		return nil, fmt.Errorf("failed to get DXP allowance: %v", err)
	}
	statusInfo["dxpAllowance"] = dxpAllowance
	
	return statusInfo, nil
}

// GetVerifierStake returns the current stake of the verifier
func (c *Client) GetVerifierStake() (*big.Int, error) {
	// Create call options
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}

	// Try different function names based on the contract
	functionNames := []string{"getVerifierStake", "verifierStake"}

	var stake *big.Int
	var err error

	for _, funcName := range functionNames {
		var out []interface{}
		err = c.contractBound.Call(callOpts, &out, funcName, c.walletAddress)
		if err == nil && len(out) > 0 {
			// Convert the output to a big.Int
			stakeVal, ok := out[0].(*big.Int)
			if ok {
				stake = stakeVal
				return stake, nil
			}
		}
	}

	// If we couldn't get the stake through direct calls, try the mapping
	var out []interface{}
	err = c.contractBound.Call(callOpts, &out, "verifierStakes", c.walletAddress)
	if err == nil && len(out) > 0 {
		// Convert the output to a big.Int
		stakeVal, ok := out[0].(*big.Int)
		if ok {
			stake = stakeVal
			return stake, nil
		}
	}

	// If all else fails, just return 0
	return big.NewInt(0), nil
}

// WithdrawVerifierStake withdraws the specified amount of stake from the verifier
func (c *Client) WithdrawVerifierStake(amount *big.Int) (string, error) {
	fmt.Printf("Withdrawing %s tokens from verifier stake...\n", formatTokenAmount(amount))

	// Check if the verifier is registered
	registered, err := c.IsRegisteredVerifier()
	if err != nil {
		return "", fmt.Errorf("failed to check if registered: %v", err)
	}
	if !registered {
		return "", fmt.Errorf("not registered as a verifier")
	}

	// Get current stake
	currentStake, err := c.GetVerifierStake()
	if err != nil {
		return "", fmt.Errorf("failed to get current stake: %v", err)
	}

	// Check if trying to withdraw more than current stake
	if amount.Cmp(currentStake) > 0 {
		return "", fmt.Errorf("insufficient stake: have %s, trying to withdraw %s",
			formatTokenAmount(currentStake), formatTokenAmount(amount))
	}

	// Get minimum stake
	minStake, err := c.GetMinVerifierStake()
	if err != nil {
		return "", fmt.Errorf("failed to get minimum stake: %v", err)
	}

	// Calculate remaining stake after withdrawal
	remainingStake := new(big.Int).Sub(currentStake, amount)

	// If this is a partial withdrawal that would leave stake below minimum, prevent it
	// But allow full withdrawal (remainingStake == 0)
	if remainingStake.Cmp(big.NewInt(0)) > 0 && remainingStake.Cmp(minStake) < 0 {
		return "", fmt.Errorf("withdrawal would put stake below minimum requirement (%s DXP). Either withdraw less or withdraw all stake", formatTokenAmount(minStake))
	}

	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return "", fmt.Errorf("failed to create transaction options: %v", err)
	}

	// Pack the data for the withdrawVerifierStake function
	data, err := c.contractABI.Pack("withdrawVerifierStake", c.walletAddress, amount)
	if err != nil {
		return "", fmt.Errorf("failed to pack data: %v", err)
	}

	// Send transaction
	tx, err := c.sendTransaction(auth, data)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	// Wait for the withdrawal transaction to be mined
	_, err = c.WaitForTransaction(tx.Hash().Hex())
	if err != nil {
		return tx.Hash().Hex(), fmt.Errorf("error waiting for transaction: %v", err)
	}

	// If we're withdrawing all stake, inform the user
	if remainingStake.Cmp(big.NewInt(0)) == 0 {
		fmt.Println("You have withdrawn all your stake and are no longer registered as a verifier.")
	} else if remainingStake.Cmp(minStake) < 0 {
		fmt.Printf("Warning: Your remaining stake (%s DXP) is below the minimum requirement (%s DXP).\n", 
			formatTokenAmount(remainingStake), formatTokenAmount(minStake))
		fmt.Println("You are still registered in the contract but will not be considered an active verifier.")
	}

	return tx.Hash().Hex(), nil
}

// ApproveDXPToken approves the DXP contract to spend tokens
func (c *Client) ApproveDXPToken(amount *big.Int) (string, error) {
	fmt.Printf("Approving DXP contract to spend %s tokens...\n", formatTokenAmount(amount))

	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return "", err
	}

	// Create transaction data for approve function
	data, err := c.tokenABI.Pack(
		"approve",
		c.contractAddress, // spender
		amount,            // amount
	)
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %w", err)
	}

	// Send transaction to token contract
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(11155111),
		Nonce:     auth.Nonce.Uint64(),
		GasFeeCap: auth.GasFeeCap,
		GasTipCap: auth.GasTipCap,
		Gas:       auth.GasLimit,
		To:        &c.tokenAddress,
		Value:     big.NewInt(0),
		Data:      data,
	})

	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(11155111)), c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	err = c.ethClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx.Hash().Hex(), nil
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

	// Get gas tip cap (priority fee)
	gasTipCap, err := c.ethClient.SuggestGasTipCap(context.Background())
	if err != nil {
		// If we can't get a suggested tip cap, set a reasonable default
		gasTipCap = big.NewInt(1500000000) // 1.5 Gwei
		fmt.Printf("Warning: Failed to get suggested gas tip cap, using default: %s wei\n", gasTipCap.String())
	}

	// Ensure minimum tip cap of 1 wei
	if gasTipCap.Cmp(big.NewInt(1)) < 0 {
		gasTipCap = big.NewInt(1500000000) // 1.5 Gwei
	}

	// Calculate gas fee cap (base fee + tip cap)
	gasFeeCap := new(big.Int).Add(
		gasPrice,
		new(big.Int).Mul(gasTipCap, big.NewInt(2)), // Add 2x the tip as buffer
	)

	// Create transaction options
	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, big.NewInt(11155111))
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %v", err)
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.GasTipCap = gasTipCap
	auth.GasFeeCap = gasFeeCap
	auth.GasLimit = 500000

	return auth, nil
}

// Helper function to send a transaction
func (c *Client) sendTransaction(auth *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	// Create transaction
	// Make sure GasTipCap is not higher than GasFeeCap
	gasTipCap := auth.GasTipCap
	if gasTipCap == nil {
		gasTipCap = big.NewInt(1500000000) // 1.5 gwei tip
	}

	gasFeeCap := auth.GasFeeCap
	if gasFeeCap == nil {
		// Get gas price
		gasPrice, err := c.ethClient.SuggestGasPrice(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get gas price: %v", err)
		}
		gasFeeCap = new(big.Int).Mul(gasPrice, big.NewInt(2)) // 2x gas price as fee cap
	}

	// If GasTipCap is higher than GasFeeCap, set it to 1/10 of GasFeeCap
	if gasFeeCap.Cmp(big.NewInt(0)) > 0 && gasTipCap.Cmp(gasFeeCap) > 0 {
		gasTipCap = new(big.Int).Div(gasFeeCap, big.NewInt(10))
	}

	// Ensure we have a value
	value := auth.Value
	if value == nil {
		value = big.NewInt(0)
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(11155111),
		Nonce:     auth.Nonce.Uint64(),
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       auth.GasLimit,
		To:        &c.contractAddress,
		Value:     value,
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

			// Check transaction status
			if receipt.Status == 0 {
				return receipt, fmt.Errorf("transaction failed: execution reverted")
			}

			return receipt, nil
		} else if err != ethereum.NotFound {
			return nil, fmt.Errorf("failed to get transaction receipt: %v", err)
		}

		fmt.Printf("Transaction not yet mined, waiting 5 seconds...\n")
		time.Sleep(5 * time.Second)
	}

	return nil, fmt.Errorf("transaction not mined within timeout")
}

// FormatTokenAmount formats a token amount with 18 decimals to a human-readable string
func (c *Client) FormatTokenAmount(amount *big.Int) string {
	return formatTokenAmount(amount)
}

// formatTokenAmount formats a token amount with 18 decimals to a human-readable string
func formatTokenAmount(amount *big.Int) string {
	if amount == nil {
		return "0"
	}

	// Convert to a big.Float
	amountFloat := new(big.Float).SetInt(amount)

	// Divide by 10^18 (18 decimals)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	amountFloat = amountFloat.Quo(amountFloat, divisor)

	// Format with 4 decimal places
	return amountFloat.Text('f', 4)
}

// IsVerifier checks if the given address is a verifier
func (c *Client) IsVerifier(address string) (bool, error) {
	// Create call options
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}

	// Call isVerifier function
	var out []interface{}
	err := c.contractBound.Call(callOpts, &out, "isVerifier", common.HexToAddress(address))
	if err != nil {
		return false, fmt.Errorf("failed to check if verifier: %v", err)
	}

	if len(out) == 0 {
		return false, nil
	}

	isVerifier, ok := out[0].(bool)
	if !ok {
		return false, fmt.Errorf("invalid verifier type")
	}

	return isVerifier, nil
}

// GetWalletAddress returns the wallet address as a hex string
func (c *Client) GetWalletAddress() string {
	return c.walletAddress.Hex()
}

// GetContractAddress returns the contract address as a hex string
func (c *Client) GetContractAddress() string {
	return c.contractAddress.Hex()
}

// CheckAndUpdateVerifierStatus calls the contract's checkAndUpdateVerifierStatus function
func (c *Client) CheckAndUpdateVerifierStatus(address string) (bool, error) {
	// Create transaction options
	auth, err := c.createTransactOpts()
	if err != nil {
		return false, fmt.Errorf("failed to create transaction options: %v", err)
	}

	// Call checkAndUpdateVerifierStatus function
	data, err := c.contractABI.Pack("checkAndUpdateVerifierStatus", common.HexToAddress(address))
	if err != nil {
		return false, fmt.Errorf("failed to pack data: %v", err)
	}

	// Send transaction
	tx, err := c.sendTransaction(auth, data)
	if err != nil {
		return false, fmt.Errorf("failed to send transaction: %v", err)
	}

	// Wait for transaction to be mined
	_, err = c.WaitForTransaction(tx.Hash().Hex())
	if err != nil {
		return false, fmt.Errorf("transaction failed: %v", err)
	}

	// Check if the verifier is registered after the update
	return c.IsVerifier(address)
}

// GetETHBalance returns the ETH balance of the wallet
func (c *Client) GetETHBalance() (*big.Int, error) {
	balance, err := c.ethClient.BalanceAt(context.Background(), c.walletAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH balance: %v", err)
	}

	return balance, nil
}
