package consensus

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/dexponent/dxp-verifier/pkg/eth"
)

type ContractConsensus struct {
	ethClient    *eth.Client
	ctx          context.Context
	cancel       context.CancelFunc
	farmIDs      []int64
	activeFarms  map[uint64]bool
	activeRounds map[uint64]uint64
	mutex        sync.RWMutex
}

func NewContractConsensus(ethClient *eth.Client) *ContractConsensus {
	ctx, cancel := context.WithCancel(context.Background())
	return &ContractConsensus{
		ethClient:    ethClient,
		ctx:          ctx,
		cancel:       cancel,
		activeFarms:  make(map[uint64]bool),
		activeRounds: make(map[uint64]uint64),
		mutex:        sync.RWMutex{},
	}
}

func (c *ContractConsensus) Start() error {
	// Get assigned farms
	var err error
	c.farmIDs, err = c.ethClient.GetAssignedFarms()
	if err != nil {
		return fmt.Errorf("failed to get assigned farms: %v", err)
	}

	if len(c.farmIDs) == 0 {
		return fmt.Errorf("no farms assigned to this verifier")
	}

	
	eventCh, err := c.ethClient.WatchForConsensusEvents(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to watch for consensus events: %v", err)
	}

	
	go c.processConsensusEvents(eventCh)

	
	go c.checkExistingRounds()

	return nil
}

func (c *ContractConsensus) Stop() {
	c.cancel()
}

func (c *ContractConsensus) processConsensusEvents(eventCh <-chan interface{}) {
	doneCtx := c.ctx.Done()
	for {
		select {
		case <-doneCtx:
			return
		case event, ok := <-eventCh:
			if !ok {
				
				log.Println("Event channel closed, restarting subscription...")
				
				time.Sleep(5 * time.Second)
				newEventCh, err := c.ethClient.WatchForConsensusEvents(c.ctx)
				if err != nil {
					log.Printf("Failed to resubscribe to events: %v\n", err)
					return
				}
				go c.processConsensusEvents(newEventCh)
				return
			}
			switch e := event.(type) {
			case eth.RoundStartedEvent:
				c.handleRoundStarted(e)
			case eth.RoundFinalizedEvent:
				c.handleRoundFinalized(e)
			default:
				log.Printf("Unknown event type received: %T\n", e)
			}
		}
	}
}

func (c *ContractConsensus) handleRoundStarted(event eth.RoundStartedEvent) {
	
	farmFound := false
	for _, farmID := range c.farmIDs {
		if uint64(farmID) == event.FarmID {
			farmFound = true
			break
		}
	}

	if !farmFound {
		return
	}

	log.Printf("Round %d started for farm %d at block %d\n", 
		event.RoundID, event.FarmID, event.StartBlock)

	
	c.mutex.Lock()
	c.activeFarms[event.FarmID] = true
	c.activeRounds[event.FarmID] = event.RoundID
	c.mutex.Unlock()

	
	go c.calculateAndSubmitScores(event.FarmID, event.RoundID)
}

func (c *ContractConsensus) checkExistingRounds() {
	consensusAddr := c.ethClient.GetConsensusAddress()
	log.Printf("Listening for consensus events at %s\n", consensusAddr)

	// Create a ticker to periodically check
	statusTicker := time.NewTicker(30 * time.Second)
	defer statusTicker.Stop()

	// Check immediately on startup without verbose logging
	c.checkAllFarms(false)

	// Then periodically check with logs
	for {
		select {
		case <-statusTicker.C:
			// Let the checkAllFarms function handle the logging
			c.checkAllFarms(true)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *ContractConsensus) checkAllFarms(verbose bool) {
	consensusAddr := c.ethClient.GetConsensusAddress()
	
	if verbose {
		log.Printf("Checking for active rounds at consensus contract %s\n", consensusAddr)
	}
	
	for _, farmID := range c.farmIDs {
		active, roundID, err := c.ethClient.GetRoundStatus(uint64(farmID))
		if err != nil {
			log.Printf("Failed to check round status for farm %d: %v\n", farmID, err)
			continue
		}

		if active {
			log.Printf("Found active round %d for farm %d\n", roundID, farmID)
			
			c.mutex.Lock()
			c.activeFarms[uint64(farmID)] = true
			c.activeRounds[uint64(farmID)] = roundID
			c.mutex.Unlock()

			go c.calculateAndSubmitScores(uint64(farmID), roundID)
		} else if verbose {
			// Only log the "no active round" message when in verbose mode
			log.Printf("No active round found for farm %d\n", farmID)
		}
	}
}

func (c *ContractConsensus) calculateAndSubmitScores(farmID, roundID uint64) {
	
	active, currentRoundID, err := c.ethClient.GetRoundStatus(farmID)
	if err != nil {
		log.Printf("Failed to check round status: %v\n", err)
		return
	}

	
	if !active || currentRoundID != roundID {
		log.Printf("Round %d for farm %d is no longer active\n", roundID, farmID)
		c.mutex.Lock()
		delete(c.activeFarms, farmID)
		c.mutex.Unlock()
		return
	}

	
	farmReturns := generateFarmReturns(20)
	
	
	score := calculateFarmScore(farmReturns)
	
	
	benchmark := calculateBenchmarkScore()
	
	log.Printf("Calculated score %.4f and benchmark %.2f%% for farm %d round %d\n", 
		score, benchmark, farmID, roundID)
	
	
	tx, err := c.ethClient.SubmitScoreAndBenchmark(farmID, score, benchmark)
	if err != nil {
		log.Printf("Failed to submit score and benchmark: %v\n", err)
		return
	}
	
	log.Printf("Submitted score and benchmark for farm %d round %d, tx: %s\n", 
		farmID, roundID, tx.Hash().Hex())
}

func generateFarmReturns(length int) []float64 {
	returns := make([]float64, length)
	for i := 0; i < length; i++ {
		returns[i] = 0.1 + float64(i%10) * 0.5
		returns[i] = math.Round(returns[i]*100) / 100
	}
	return returns
}

func calculateBenchmarkScore() float64 {
	
	currentPct := 10.0

	if currentPct < 5.0 {
		currentPct = 5.0
	} else if currentPct > 15.0 {
		currentPct = 15.0
	}

	return math.Round(currentPct*100) / 100
}

func calculateFarmScore(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	normalizedYield := sum / float64(len(returns))

	volumeWeight := math.Log10(float64(len(returns)) + 1)

	var variance float64
	if len(returns) > 1 {
		for _, r := range returns {
			variance += math.Pow(r-normalizedYield, 2)
		}
		variance /= float64(len(returns))
	}
	stdDev := math.Sqrt(variance)

	sharpeRatio := 1.0
	if stdDev > 0 {
		sharpeRatio = normalizedYield / stdDev
	}
	var downside float64
	for _, r := range returns {
		if r < 0 {
			downside += r * r
		}
	}
	sortinoRatio := 1.0
	if downside > 0 {
		sortinoRatio = normalizedYield / math.Sqrt(downside/float64(len(returns)))
	}

	consistencyFactor := 1.0
	if len(returns) > 1 {
		consistencyFactor = 1.0 / (1.0 + variance)
	}
	farmScore := (normalizedYield * volumeWeight) * (0.5*sharpeRatio + 0.5*sortinoRatio) * consistencyFactor

	if farmScore > 1.0 {
		farmScore = 1.0
	}

	return math.Round(farmScore*10000) / 10000
}

func (c *ContractConsensus) handleRoundFinalized(event eth.RoundFinalizedEvent) {
	farmFound := false
	for _, farmID := range c.farmIDs {
		if uint64(farmID) == event.FarmID {
			farmFound = true
			break
		}
	}

	if !farmFound {
		return
	}

	log.Printf("Round %d finalized for farm %d with consensus score %.4f and benchmark %.2f%%\n", 
		event.RoundID, event.FarmID, float64(event.ConsensusScore)/10000, float64(event.ConsensusBenchmark)/100)

	c.mutex.Lock()
	delete(c.activeFarms, event.FarmID)
	if c.activeRounds[event.FarmID] == event.RoundID {
		delete(c.activeRounds, event.FarmID)
	}
	c.mutex.Unlock()
}
