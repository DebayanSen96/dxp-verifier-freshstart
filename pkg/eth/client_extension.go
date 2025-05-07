package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/core/types"
)

// FinalizeConsensusRound finalizes the active consensus round for a farm
// This function can only be called by the contract owner
func (c *Client) FinalizeConsensusRound(farmID int64) (*types.Transaction, error) {
	// Create the method signature for finalizeRound in Consensus contract
	methodSig := []byte("finalizeRound(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the farmID parameter
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

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

// StartConsensusRound starts a new consensus round for a farm
// This function can only be called by the contract owner
func (c *Client) StartConsensusRound(farmID int64) (*types.Transaction, error) {
	// Create the method signature for startRound in Consensus contract
	methodSig := []byte("startRound(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Pack the farmID parameter
	paddedFarmID := common.LeftPadBytes(big.NewInt(farmID).Bytes(), 32)

	// Create the call data
	data := append(methodID, paddedFarmID...)

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
