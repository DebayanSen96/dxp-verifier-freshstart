package eth

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type RoundStartedEvent struct {
	FarmID     uint64
	RoundID    uint64
	StartBlock uint64
}

type RoundFinalizedEvent struct {
	FarmID            uint64
	RoundID           uint64
	ConsensusScore    uint64
	ConsensusBenchmark uint64
}

type SubmissionReceivedEvent struct {
	FarmID     uint64
	RoundID    uint64
	Verifier   common.Address
	Score      uint64
	Benchmark  uint64
}

// GetRoundStatus checks if there's an active round for a farm
func (c *Client) GetRoundStatus(farmID uint64) (bool, uint64, error) {
	// Call the rounds mapping in the Consensus contract
	var result struct {
		ID uint64
		StartBlock uint64
		Finalized bool
	}

	// Create the method signature for rounds(uint256)
	methodSig := []byte("rounds(uint256)")
	methodID := crypto.Keccak256(methodSig)[:4]

	// Encode the farmID parameter
	paramBuf := common.LeftPadBytes(new(big.Int).SetUint64(farmID).Bytes(), 32)

	// Combine the method ID and parameters
	data := append(methodID, paramBuf...)

	// Call the contract
	result_bytes, err := c.ethClient.CallContract(context.Background(), ethereum.CallMsg{
		To:   &c.consensusAddress,
		Data: data,
	}, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to call rounds: %v", err)
	}

	// Parse the result
	if len(result_bytes) >= 96 { // 3 * 32 bytes for the three fields
		result.ID = new(big.Int).SetBytes(result_bytes[:32]).Uint64()
		result.StartBlock = new(big.Int).SetBytes(result_bytes[32:64]).Uint64()
		result.Finalized = new(big.Int).SetBytes(result_bytes[64:96]).Uint64() > 0
	}

	// Return true if there's an active round (ID > 0 and not finalized)
	return (result.ID > 0 && !result.Finalized), result.ID, nil
}

// WatchForConsensusEvents watches for both RoundStarted and RoundFinalized events
func (c *Client) WatchForConsensusEvents(ctx context.Context) (<-chan interface{}, error) {
	eventCh := make(chan interface{})
	
	// Get the current block number to start polling from
	currentBlock, err := c.ethClient.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current block number: %v", err)
	}
	
	// Create filter queries for both event types
	roundStartedSig := crypto.Keccak256Hash([]byte("RoundStarted(uint256,uint256,uint256)"))
	roundFinalizedSig := crypto.Keccak256Hash([]byte("RoundFinalized(uint256,uint256,uint256,uint256)"))
	
	// Use polling instead of subscription
	go func() {
		defer close(eventCh)
		
		// Keep track of the last block we checked
		lastCheckedBlock := currentBlock
		
		// Poll for new events every 10 seconds
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Get the current block number
				newBlock, err := c.ethClient.BlockNumber(ctx)
				if err != nil {
					fmt.Printf("Error getting current block number: %v\n", err)
					continue
				}
				
				// If no new blocks, continue
				if newBlock <= lastCheckedBlock {
					continue
				}
				
				// Check for RoundStarted events
				startQuery := ethereum.FilterQuery{
					Addresses: []common.Address{c.consensusAddress},
					Topics: [][]common.Hash{
						{roundStartedSig},
					},
					FromBlock: new(big.Int).SetUint64(lastCheckedBlock + 1),
					ToBlock: new(big.Int).SetUint64(newBlock),
				}
				
				startLogs, err := c.ethClient.FilterLogs(ctx, startQuery)
				if err != nil {
					fmt.Printf("Error filtering RoundStarted logs: %v\n", err)
				} else {
					// Process the RoundStarted logs
					for _, log := range startLogs {
						if len(log.Topics) < 4 {
							continue
						}
						
						farmID := new(big.Int).SetBytes(log.Topics[1][:]).Uint64()
						roundID := new(big.Int).SetBytes(log.Topics[2][:]).Uint64()
						startBlock := new(big.Int).SetBytes(log.Topics[3][:]).Uint64()
						
						event := RoundStartedEvent{
							FarmID:     farmID,
							RoundID:    roundID,
							StartBlock: startBlock,
						}
						
						fmt.Printf("Detected RoundStarted event: Farm %d, Round %d\n", farmID, roundID)
						
						// Send the event to the channel
						select {
						case eventCh <- event:
						case <-ctx.Done():
							return
						}
					}
				}
				
				// Check for RoundFinalized events
				finalizedQuery := ethereum.FilterQuery{
					Addresses: []common.Address{c.consensusAddress},
					Topics: [][]common.Hash{
						{roundFinalizedSig},
					},
					FromBlock: new(big.Int).SetUint64(lastCheckedBlock + 1),
					ToBlock: new(big.Int).SetUint64(newBlock),
				}
				
				finalizedLogs, err := c.ethClient.FilterLogs(ctx, finalizedQuery)
				if err != nil {
					fmt.Printf("Error filtering RoundFinalized logs: %v\n", err)
				} else {
					// Process the RoundFinalized logs
					for _, log := range finalizedLogs {
						if len(log.Topics) < 3 {
							continue
						}
						
						farmID := new(big.Int).SetBytes(log.Topics[1][:]).Uint64()
						roundID := new(big.Int).SetBytes(log.Topics[2][:]).Uint64()
						
						// Decode the non-indexed parameters
						var consensusScore, consensusBenchmark uint64
						if len(log.Data) >= 64 {
							consensusBenchmark = new(big.Int).SetBytes(log.Data[32:64]).Uint64()
							consensusScore = new(big.Int).SetBytes(log.Data[0:32]).Uint64()
						}
						
						event := RoundFinalizedEvent{
							FarmID:            farmID,
							RoundID:           roundID,
							ConsensusScore:    consensusScore,
							ConsensusBenchmark: consensusBenchmark,
						}
						
						fmt.Printf("Detected RoundFinalized event: Farm %d, Round %d\n", farmID, roundID)
						
						// Send the event to the channel
						select {
						case eventCh <- event:
						case <-ctx.Done():
							return
						}
					}
				}
				
				// Update the last checked block
				lastCheckedBlock = newBlock
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return eventCh, nil
}

// For backward compatibility
func (c *Client) WatchForRoundStartEvents(ctx context.Context) (<-chan RoundStartedEvent, error) {
	allEventsCh, err := c.WatchForConsensusEvents(ctx)
	if err != nil {
		return nil, err
	}
	
	// Create a channel specifically for RoundStarted events
	roundStartedCh := make(chan RoundStartedEvent)
	
	// Forward only RoundStarted events
	go func() {
		defer close(roundStartedCh)
		
		for {
			select {
			case event, ok := <-allEventsCh:
				if !ok {
					return
				}
				
				// Check if it's a RoundStarted event
				if startEvent, ok := event.(RoundStartedEvent); ok {
					select {
					case roundStartedCh <- startEvent:
					case <-ctx.Done():
						return
					}
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return roundStartedCh, nil
}

// This method has been moved to the top of the file

func (c *Client) SubmitScoreAndBenchmark(farmID uint64, score, benchmark float64) (*types.Transaction, error) {
	// Create big.Float values for the existing SubmitVerification method
	scoreFloat := new(big.Float).SetFloat64(score)
	benchmarkFloat := new(big.Float).SetFloat64(benchmark)
	
	return c.SubmitVerification(int64(farmID), scoreFloat, benchmarkFloat)
}
