package eth

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// SetFarmBenchmarkSecure sets the benchmark for a farm with cryptographic proof
// It uses ECDSA signature to ensure only authorized verifiers can submit benchmarks
func (c *Client) SetFarmBenchmarkSecure(farmID int64, benchmark *big.Int) (*types.Transaction, error) {
	// Create signature for the benchmark data
	signature, err := c.signBenchmarkData(farmID, benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to sign benchmark data: %v", err)
	}

	// Use proper ABI encoding for the function call with dynamic bytes parameter
	abi, err := abi.JSON(strings.NewReader(`[
		{
			"name": "setFarmBenchmarkSecure",
			"type": "function",
			"inputs": [
				{"name": "farmId", "type": "uint256"},
				{"name": "benchmark", "type": "uint256"},
				{"name": "signature", "type": "bytes"}
			]
		}
	]`))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %v", err)
	}

	// Pack the parameters using the ABI
	data, err := abi.Pack("setFarmBenchmarkSecure", big.NewInt(farmID), benchmark, signature)
	if err != nil {
		return nil, fmt.Errorf("failed to pack parameters: %v", err)
	}

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

// signBenchmarkData creates a cryptographic signature for the benchmark data
// This function precisely replicates the hashing and signing required by the
// Consensus contract's setFarmBenchmarkSecure function.
func (c *Client) signBenchmarkData(farmID int64, benchmark *big.Int) ([]byte, error) {
	// Get the verifier address from the client's private key
	// Use crypto.PubkeyToAddress to derive the public address correctly
	address := crypto.PubkeyToAddress(c.privateKey.PublicKey)

	// Step 1: Replicate abi.encodePacked(farmId, benchmark, msg.sender)
	packedData := []byte{}

	// Pack farmId (uint256) - Big-endian, 32 bytes
	farmIdBytes := make([]byte, 32)
	big.NewInt(farmID).FillBytes(farmIdBytes)
	packedData = append(packedData, farmIdBytes...)

	// Pack benchmark (uint256) - Big-endian, 32 bytes
	benchmarkBytes := make([]byte, 32)
	benchmark.FillBytes(benchmarkBytes)
	packedData = append(packedData, benchmarkBytes...)

	// Pack address (address) - 20 bytes (no padding in abi.encodePacked)
	addressBytes := address.Bytes()
	packedData = append(packedData, addressBytes...)

	fmt.Printf("DEBUG: ABI packed message: %x\n", packedData)

	// Step 2: Calculate the keccak256 hash of the packed data
	// This matches: bytes32 messageHash = keccak256(abi.encodePacked(...));
	messageHash := crypto.Keccak256(packedData)
	fmt.Printf("DEBUG: Message hash: %x\n", messageHash)

	// Step 3: Apply the Ethereum Signed Message prefix
	// This matches: bytes32 ethSignedMessageHash = MessageHashUtils.toEthSignedMessageHash(messageHash);
	msgLen := fmt.Sprintf("%d", len(messageHash))
	prefixedMessage := []byte("\x19Ethereum Signed Message:\n" + msgLen)
	prefixedMessage = append(prefixedMessage, messageHash...)
	prefixedHash := crypto.Keccak256(prefixedMessage)
	fmt.Printf("DEBUG: Prefixed hash: %x\n", prefixedHash)

	// Step 4: Sign the prefixed hash with the private key
	signature, err := crypto.Sign(prefixedHash, c.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %v", err)
	}

	// Step 5: Format the signature for Ethereum compatibility
	// Extract R, S, V components
	r := signature[:32]
	s := new(big.Int).SetBytes(signature[32:64])
	v := signature[64] // 0 or 1 from Go's crypto.Sign

	// Ensure S is in lower half order as required by OpenZeppelin's ECDSA implementation
	curve := crypto.S256()
	halfOrder := new(big.Int).Rsh(curve.Params().N, 1)

	if s.Cmp(halfOrder) > 0 {
		// If S > half order, set S = N - S (where N is the curve order)
		s = new(big.Int).Sub(curve.Params().N, s)
		// Flip V (0->1, 1->0) when S is adjusted
		v = 1 - v
	}

	// Format S to 32 bytes
	sBytes := make([]byte, 32)
	tempS := s.Bytes()
	copy(sBytes[32-len(tempS):], tempS)

	// Adjust V to Ethereum format (27 or 28)
	v += 27

	// Construct the final signature [R || S || V]
	finalSig := make([]byte, 65)
	copy(finalSig[:32], r)
	copy(finalSig[32:64], sBytes)
	finalSig[64] = v

	fmt.Printf("DEBUG: Signature: %x\n", finalSig)

	return finalSig, nil
}
