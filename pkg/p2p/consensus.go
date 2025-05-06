package p2p

import (
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// generateFarmReturns generates random farm returns for testing
// In a real implementation, this would fetch returns from a blockchain
func generateFarmReturns(length int) []float64 {
	returns := make([]float64, length)
	for i := 0; i < length; i++ {
		// Generate a random return between 0.1 and 10.0
		returns[i] = 0.1 + rand.Float64()*9.9
		// Round to 2 decimal places
		returns[i] = math.Round(returns[i]*100) / 100
	}
	return returns
}

func calculateBenchmarkScore() float64 {
	// Start with a base benchmark of 10%
	currentPct := 10.0

	// Random variation between -4% and +4% of the current value
	variation := (rand.Float64()*8.0 - 4.0) / 100.0
	newPct := currentPct * (1.0 + variation)

	// Ensure it stays within reasonable bounds (5-15%)
	if newPct < 5.0 {
		newPct = 5.0
	} else if newPct > 15.0 {
		newPct = 15.0
	}

	// Round to 2 decimal places for consistency
	return math.Round(newPct*100) / 100
}

// calculateFarmScore calculates a farm score based on returns
func calculateFarmScore(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	// Calculate normalized yield (average return)
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	normalizedYield := sum / float64(len(returns))

	// Calculate volume weight (simplified)
	volumeWeight := math.Log10(float64(len(returns)) + 1)

	// Calculate standard deviation for Sharpe ratio
	var variance float64
	if len(returns) > 1 {
		for _, r := range returns {
			variance += math.Pow(r-normalizedYield, 2)
		}
		variance /= float64(len(returns))
	}
	stdDev := math.Sqrt(variance)

	// Calculate Sharpe ratio (assuming risk-free rate of 0 for simplicity)
	sharpeRatio := 1.0
	if stdDev > 0 {
		sharpeRatio = normalizedYield / stdDev
	}

	// Calculate Sortino ratio (simplified)
	// In a real implementation, this would be more complex
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

	// Calculate consistency factor
	consistencyFactor := 1.0
	if len(returns) > 1 {
		// Higher consistency (lower variance) gives higher factor
		consistencyFactor = 1.0 / (1.0 + variance)
	}

	// Calculate final score - now incorporating both Sharpe and Sortino ratios
	// We weight them equally in this implementation
	farmScore := (normalizedYield * volumeWeight) * (0.5*sharpeRatio + 0.5*sortinoRatio) * consistencyFactor

	// Cap the score at 1.0
	if farmScore > 1.0 {
		farmScore = 1.0
	}

	// Round to 4 decimal places
	return math.Round(farmScore*10000) / 10000
}

// calculateMedian calculates the median of a slice of float64
func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0.0 // Or handle as an error, depending on requirements
	}
	sort.Float64s(values) // Sort the slice

	n := len(values)
	if n%2 == 0 {
		// Even number of elements, median is the average of the two middle elements
		mid1 := values[n/2-1]
		mid2 := values[n/2]
		return (mid1 + mid2) / 2.0
	}
	// Odd number of elements, median is the middle element
	return values[n/2]
}

// selectLeader deterministically selects a leader from the list of peers
func (p *DexponentProtocol) selectLeader() (peer.ID, bool) {
	peers := p.GetDexponentPeers()

	// Add our own ID to the list
	allPeers := append([]peer.ID{}, peers...) // Create a copy of peers
	allPeers = append(allPeers, p.host.ID())

	// Need at least 3 peers for consensus
	if len(allPeers) < 3 {
		return "", false
	}

	// Sort peer IDs lexicographically to ensure everyone gets the same order
	sort.Slice(allPeers, func(i, j int) bool {
		return allPeers[i].String() < allPeers[j].String()
	})

	// Use next round number to rotate leadership
	// We use currentRound + 1 because this function is called before incrementing the round
	leaderIndex := (p.currentRound + 1) % int64(len(allPeers))
	leader := allPeers[leaderIndex]

	// Check if we are the leader
	isLeader := leader == p.host.ID()

	return leader, isLeader
}

// StartConsensusProcess initiates the consensus process if enough peers are connected
func (p *DexponentProtocol) StartConsensusProcess() {
	// Check if we have an ethClient to get farm assignments
	if p.ethClient != nil {
		// Get assigned farms
		farms, err := p.ethClient.GetAssignedFarms()
		if err == nil && len(farms) > 0 {
			// We have farm assignments, use farm-specific consensus instead
			return
		}
	}
	// Use stateLock to safely check and update consensus state
	p.stateLock.RLock()

	// Check for stalled rounds - if a round has been active for more than 1.5 minutes, it's likely stalled
	if p.roundActive && time.Since(p.roundStartTime) > 90*time.Second {
		fmt.Printf("⚠️ Detected stalled consensus round %d. Forcing reset...\n", p.currentRound)
		p.stateLock.RUnlock()

		// Force reset the round state
		p.stateLock.Lock()
		p.roundActive = false
		// Set a short cooldown to allow the system to stabilize
		p.cooldownEndTime = time.Now().Add(5 * time.Second)
		p.stateLock.Unlock()
		return
	}

	// Check if we're already in an active round
	if p.roundActive {
		p.stateLock.RUnlock()
		return
	}

	// Check cooldown period with a small tolerance for clock differences
	if time.Now().Before(p.cooldownEndTime.Add(-1 * time.Second)) {
		// We're still in cooldown, don't start a new round yet
		p.stateLock.RUnlock()
		return
	}
	p.stateLock.RUnlock()

	// Select a leader for the next round
	leader, isLeader := p.selectLeader()

	// Update our state with write lock
	p.stateLock.Lock()
	p.isLeader = isLeader
	p.currentLeader = leader

	// If we're not the leader, don't start a consensus round
	if !isLeader {
		p.stateLock.Unlock()
		return
	}

	// We are the leader for this round, increment the round number
	p.currentRound++

	// Generate farm returns
	p.farmReturns = generateFarmReturns(20)
	p.consensusBenchmark = calculateBenchmarkScore()
	p.stateLock.Unlock()

	// Broadcast leader election message
	leaderElectionPayload := LeaderElectionPayload{
		LeaderID:    p.host.ID().String(),
		RoundNumber: p.currentRound,
	}

	// Broadcast to all peers
	p.BroadcastMessage(MessageTypeLeaderElection, leaderElectionPayload)

	// Wait a moment for the leader election message to propagate
	time.Sleep(2 * time.Second)

	// Start the consensus round
	p.startConsensusRound()
}

// startConsensusRound starts a new consensus round
func (p *DexponentProtocol) startConsensusRound() {
	// Set round timing with a small buffer to ensure all nodes have time to process
	// the start message before beginning score calculations
	startTime := time.Now().Add(2 * time.Second)
	roundDuration := 60 * time.Second
	endTime := startTime.Add(roundDuration)

	// Update protocol state
	p.roundActive = true
	p.roundStartTime = startTime
	p.roundEndTime = endTime
	p.scoresLock.Lock()
	p.scores = make(map[peer.ID]float64)
	// Add our own score
	p.scores[p.host.ID()] = calculateFarmScore(p.farmReturns)
	p.scoresLock.Unlock()

	p.benchmarksLock.Lock()                               // Lock benchmarks map
	p.benchmarks = make(map[peer.ID]float64)              // Initialize benchmarks map
	p.benchmarks[p.host.ID()] = calculateBenchmarkScore() // Add leader's own benchmark
	p.benchmarksLock.Unlock()                             // Unlock benchmarks map

	// Create consensus start payload
	consensusStartPayload := ConsensusStartPayload{
		RoundNumber: p.currentRound,
		FarmReturns: p.farmReturns,
		StartTime:   startTime.Unix(),
		EndTime:     endTime.Unix(),
		// For global consensus, we use farm ID 0
		FarmID: 0,
	}

	// Broadcast consensus start message
	fmt.Printf("🚀 Starting consensus round %d as leader for global farm (ID: 0). Round will end in %v.\n", p.currentRound, roundDuration)
	p.BroadcastMessage(MessageTypeConsensusStart, consensusStartPayload)

	// Schedule the end of the round
	time.AfterFunc(roundDuration, func() {
		p.finalizeConsensusRound()
	})
}

// finalizeConsensusRound finalizes the consensus round and broadcasts results
func (p *DexponentProtocol) finalizeConsensusRound() {
	// Use stateLock to safely check and update consensus state
	p.stateLock.RLock()
	// Only the leader should finalize the round
	roundToEnd := p.currentRound
	if !p.isLeader || !p.roundActive || roundToEnd == 0 { // Add check for round 0
		p.stateLock.RUnlock()
		return
	}
	p.stateLock.RUnlock()

	// Update round state with write lock
	p.stateLock.Lock()
	p.roundActive = false
	cooldownDuration := 60 * time.Second // Cooldown set to 1 minute
	p.cooldownEndTime = time.Now().Add(cooldownDuration)

	// Collect all scores
	p.scoresLock.RLock()
	scores := p.scores
	scoreValues := make([]float64, 0, len(scores))
	for _, score := range scores {
		scoreValues = append(scoreValues, score)
	}
	p.scoresLock.RUnlock()

	// Collect all benchmarks
	p.benchmarksLock.RLock()
	benchmarks := p.benchmarks
	benchmarkValues := make([]float64, 0, len(benchmarks))
	for _, benchmark := range benchmarks {
		benchmarkValues = append(benchmarkValues, benchmark)
	}
	p.benchmarksLock.RUnlock()

	// Check if we have enough scores for consensus (at least 2/3 of peers)
	peers := p.GetDexponentPeers()
	allPeers := append(peers, p.host.ID())
	requiredScores := (len(allPeers) * 2) / 3
	if len(scores) < requiredScores {
		fmt.Printf("⚠️ Not enough scores/benchmarks for consensus. Got %d, need %d\n", len(scores), requiredScores)
		p.stateLock.Unlock() // Ensure unlock before return
		return
	}

	// Calculate median score and benchmark
	finalFarmScore := calculateMedian(scoreValues)
	finalBenchmark := calculateMedian(benchmarkValues)
	p.consensusResult = finalFarmScore
	p.consensusBenchmark = finalBenchmark

	// Create list of participants
	participants := make([]string, 0, len(scores))
	for peerID := range scores {
		participants = append(participants, peerID.String())
	}

	// Create consensus result payload
	resultPayload := ConsensusResultPayload{
		RoundNumber:    roundToEnd, // Use the round number captured at the start
		FinalScore:     finalFarmScore,
		FinalBenchmark: finalBenchmark, // Include final benchmark
		Participants:   participants,
		NextRoundStart: p.cooldownEndTime.Unix(),
	}

	// Broadcast consensus result
	fmt.Printf("✅ Consensus round %d complete. Final score: %.4f, Final Benchmark: %.4f with %d participants\n",
		roundToEnd, finalFarmScore, finalBenchmark, len(participants)) // Updated log
	fmt.Printf("⏱️ Cooldown active. Next consensus round can start after %s (%v cooldown)\n",
		time.Unix(int64(p.cooldownEndTime.Unix()), 0).Format(time.RFC3339), cooldownDuration)
	p.BroadcastMessage(MessageTypeConsensusResult, resultPayload) // Fixed: Actually broadcast the message

	// Release the stateLock that was acquired at the beginning of this function
	p.stateLock.Unlock()
}

// handleLeaderElection processes a leader election message
func (p *DexponentProtocol) handleLeaderElection(stream network.Stream, msg Message) {
	// Get the remote peer ID (for logging purposes)
	_ = stream.Conn().RemotePeer()

	// Parse the payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid leader election payload format\n")
		stream.Reset()
		return
	}

	// Extract leader ID and round number
	leaderIDStr, ok := payload["leader_id"].(string)
	if !ok {
		fmt.Printf("Error: Missing leader_id in payload\n")
		stream.Reset()
		return
	}

	roundNumberFloat, ok := payload["round_number"].(float64)
	if !ok {
		fmt.Printf("Error: Missing round_number in payload\n")
		stream.Reset()
		return
	}
	roundNumber := int64(roundNumberFloat)

	// Convert leader ID string to peer.ID
	leaderID, err := peer.Decode(leaderIDStr)
	if err != nil {
		fmt.Printf("Error decoding leader ID: %v\n", err)
		stream.Reset()
		return
	}

	// Update our state with proper locking
	p.stateLock.Lock()
	p.currentRound = roundNumber
	p.currentLeader = leaderID
	p.isLeader = (leaderID == p.host.ID())
	p.stateLock.Unlock()

	// Ensure we're in a clean state for this consensus round
	if !p.isLeader {
		// Not the leader, reset our state
		p.roundActive = false
		p.scores = make(map[peer.ID]float64)
	} else {
		// We're the leader, make sure we don't have any old data
		p.scores = make(map[peer.ID]float64)
		p.roundActive = true // Mark as active since we're the leader
	}

	// Reset cooldown since we're starting a new round
	// This ensures nodes don't try to start a new round during the current one
	p.cooldownEndTime = time.Now().Add(30 * time.Second)

	fmt.Printf("📢 Received leader election for round %d. Leader: %s\n", roundNumber, leaderIDStr)

	// Close the stream
	if err := stream.Close(); err != nil {
		// Ignore "canceled" errors as they're expected during rapid stream open/close
		if !strings.Contains(err.Error(), "canceled") {
			fmt.Printf("Error closing stream after leader election: %v\n", err)
		}
	}
}

// handleConsensusStart processes a consensus start message
func (p *DexponentProtocol) handleConsensusStart(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// Parse the payload first to check for farm ID
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid consensus start payload format\n")
		stream.Reset()
		return
	}

	// Check if this is a farm-specific consensus start message
	if farmIDFloat, hasFarmID := payload["farm_id"].(float64); hasFarmID {
		// This is a farm-specific consensus start message
		farmID := int64(farmIDFloat)

		// Skip farm 0 (global consensus) messages if we have farm assignments
		if farmID == 0 && p.ethClient != nil {
			farms, err := p.ethClient.GetAssignedFarms()
			if err == nil && len(farms) > 0 {
				// We have farm assignments, ignore global consensus
				fmt.Printf("Ignoring global consensus (farm 0) - we have farm assignments\n")
				stream.Reset()
				return
			}
		}

		if farmID > 0 {
			// This is a farm-specific consensus start, redirect to handleFarmConsensusStart
			fmt.Printf("Received farm-specific consensus start for farm %d, redirecting...\n", farmID)
			p.handleFarmConsensusStart(stream, msg)
			return
		}
	}

	// This is a global consensus start message (farm ID 0 or not specified)
	// Verify this is from the current leader
	if remotePeer != p.currentLeader {
		// Silently ignore messages from non-leaders
		stream.Reset()
		return
	}

	// Extract round number and farm returns
	roundNumberFloat, ok := payload["round_number"].(float64)
	if !ok {
		fmt.Printf("Error: Missing round_number in payload\n")
		stream.Reset()
		return
	}
	roundNumber := int64(roundNumberFloat)

	farmReturnsInterface, ok := payload["farm_returns"].([]interface{})
	if !ok {
		fmt.Printf("Error: Missing farm_returns in payload\n")
		stream.Reset()
		return
	}

	// Convert farm returns to float64 slice
	farmReturns := make([]float64, len(farmReturnsInterface))
	for i, v := range farmReturnsInterface {
		farmReturns[i], ok = v.(float64)
		if !ok {
			fmt.Printf("Error: Invalid farm return value at index %d\n", i)
			stream.Reset()
			return
		}
	}

	// Extract timing information
	startTimeFloat, ok := payload["start_time"].(float64)
	if !ok {
		fmt.Printf("Error: Missing start_time in payload\n")
		stream.Reset()
		return
	}

	endTimeFloat, ok := payload["end_time"].(float64)
	if !ok {
		fmt.Printf("Error: Missing end_time in payload\n")
		stream.Reset()
		return
	}

	// Update our state
	p.roundActive = true
	p.currentRound = roundNumber
	p.farmReturns = farmReturns
	p.roundStartTime = time.Unix(int64(startTimeFloat), 0)
	p.roundEndTime = time.Unix(int64(endTimeFloat), 0)

	fmt.Printf("🔄 Received consensus start for round %d. Calculating farm score & benchmark...\n", roundNumber)

	// Calculate our farm score and benchmark
	farmScore := calculateFarmScore(farmReturns)
	farmBenchmark := calculateBenchmarkScore() // Calculate benchmark

	// Create score submission payload
	scorePayload := ScoreSubmissionPayload{
		RoundNumber:   roundNumber,
		FarmID:        0, // Set farm ID to 0 for global consensus
		FarmScore:     farmScore,
		FarmBenchmark: farmBenchmark, // Include benchmark
		SubmitterID:   p.host.ID().String(),
	}

	// Send our score to the leader
	fmt.Printf("📊 Submitting farm score %.4f and benchmark %.4f to leader for global farm (ID: 0) round %d\n", farmScore, farmBenchmark, roundNumber)
	p.SendMessageToPeer(p.currentLeader, MessageTypeScoreSubmission, scorePayload)

	// Close the stream
	if err := stream.Close(); err != nil {
		// Ignore "canceled" errors
		if !strings.Contains(err.Error(), "canceled") {
			fmt.Printf("Error closing stream after sending score: %v\n", err)
		}
	}
}

// handleScoreSubmission processes a score submission message
func (p *DexponentProtocol) handleScoreSubmission(stream network.Stream, msg Message) {
	// Parse the payload first to check for farm ID
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid payload type in score submission\n")
		stream.Reset()
		return
	}

	// Check if this is a farm-specific score submission
	if farmIDFloat, hasFarmID := payload["farm_id"].(float64); hasFarmID {
		// This is a farm-specific score submission, redirect to handleFarmScoreSubmission
		farmID := int64(farmIDFloat)
		fmt.Printf("Received farm-specific score submission for farm %d, redirecting...\n", farmID)
		p.handleFarmScoreSubmission(stream, msg)
		return
	}

	// This is a regular score submission, check if we are the global leader
	if !p.isLeader {
		// Silently ignore score submissions if we're not the leader
		fmt.Printf("Ignoring score submission - not the global leader\n")
		stream.Reset()
		return
	}

	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// We already parsed the payload above

	// Extract round number and score
	roundNumberFloat, ok := payload["round_number"].(float64)
	if !ok {
		fmt.Printf("Error: Missing round_number in payload\n")
		return
	}
	roundNumber := int64(roundNumberFloat)

	// Verify this is for the current round
	if roundNumber != p.currentRound {
		// Silently ignore scores for wrong rounds
		stream.Reset()
		return
	}

	farmScoreFloat, ok := payload["farm_score"].(float64)
	if !ok {
		fmt.Printf("Error: Missing farm_score in payload\n")
		stream.Reset()
		return
	}

	farmBenchmarkFloat, ok := payload["farm_benchmark"].(float64) // Extract benchmark
	if !ok {
		fmt.Printf("Error: Missing farm_benchmark in payload\n")
		stream.Reset()
		return
	}

	// Add the score to our collection
	p.scoresLock.Lock()
	p.scores[remotePeer] = farmScoreFloat
	scoreCount := len(p.scores)
	p.scoresLock.Unlock()

	// Add the benchmark to our collection
	p.benchmarksLock.Lock()
	p.benchmarks[remotePeer] = farmBenchmarkFloat // Store benchmark
	p.benchmarksLock.Unlock()

	fmt.Printf("📥 Received farm score %.4f & farm benchmark %.4f from %s for global farm (ID: 0) round %d (%d/%d submissions)\n",
		farmScoreFloat, farmBenchmarkFloat, remotePeer.String(), roundNumber, scoreCount, len(p.GetDexponentPeers())+1)

	// Close the stream
	if err := stream.Close(); err != nil {
		// Ignore "canceled" errors
		if !strings.Contains(err.Error(), "canceled") {
			fmt.Printf("Error closing stream after receiving score: %v\n", err)
		}
	}
}

// handleConsensusResult processes a consensus result message
func (p *DexponentProtocol) handleConsensusResult(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// Verify this is from the current leader
	if remotePeer != p.currentLeader {
		// Silently ignore messages from non-leaders
		stream.Reset()
		return
	}

	// Parse the payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid consensus result payload format\n")
		stream.Reset()
		return
	}

	// Extract round number and final score
	roundNumberFloat, ok := payload["round_number"].(float64)
	if !ok {
		fmt.Printf("Error: Missing round_number in payload\n")
		return
	}
	roundNumber := int64(roundNumberFloat)

	finalScoreFloat, ok := payload["final_score"].(float64)
	if !ok {
		fmt.Printf("Error: Missing final_score in payload\n")
		stream.Reset()
		return
	}

	// Extract final benchmark score
	finalBenchmarkFloat, ok := payload["final_benchmark"].(float64)
	if !ok {
		fmt.Printf("Warning: Missing final_benchmark in consensus result payload\n")
		finalBenchmarkFloat = 0 // Default to 0 if missing, or handle as error
	}

	// Extract participants
	participantsInterface, ok := payload["participants"].([]interface{})
	if !ok {
		fmt.Printf("Error: Missing participants in payload\n")
		stream.Reset()
		return
	}

	participants := make([]string, len(participantsInterface))
	for i, v := range participantsInterface {
		participants[i], ok = v.(string)
		if !ok {
			fmt.Printf("Error: Invalid participant format\n")
			return
		}
	}

	// Extract next round start time
	nextRoundStartFloat, ok := payload["next_round_start"].(float64)
	if !ok {
		fmt.Printf("Error: Missing next_round_start in payload\n")
		stream.Reset()
		return
	}

	// Update our state
	p.roundActive = false
	p.consensusResult = finalScoreFloat
	p.consensusBenchmark = finalBenchmarkFloat // Store final benchmark
	p.cooldownEndTime = time.Unix(int64(nextRoundStartFloat), 0)

	fmt.Printf("✅ Consensus round %d result received. Final score: %.4f, Final Benchmark: %.4f with %d participants\n",
		roundNumber, finalScoreFloat, finalBenchmarkFloat, len(participants)) // Updated log
	fmt.Printf("⏱️ Cooldown active. Next consensus round can start after %s\n",
		time.Unix(int64(nextRoundStartFloat), 0).Format(time.RFC3339))

	// Close the stream
	if err := stream.Close(); err != nil {
		// Ignore "canceled" errors
		if !strings.Contains(err.Error(), "canceled") {
			fmt.Printf("Error closing stream after receiving result: %v\n", err)
		}
	}
}

// StartFarmConsensusProcesses initiates the consensus process for each farm
func (p *DexponentProtocol) StartFarmConsensusProcesses() {
	// Check if we have an ethClient to get farm assignments
	if p.ethClient == nil {
		return
	}

	// Get assigned farms
	farms, err := p.ethClient.GetAssignedFarms()
	if err != nil {
		fmt.Printf("Error getting assigned farms: %v\n", err)
		return
	}

	// Process each farm in parallel
	for _, farmID := range farms {
		// Check if we're active for this farm
		isActive, err := p.ethClient.IsVerifierActiveForFarm(farmID)
		if err != nil || !isActive {
			continue
		}

		// Process this farm's consensus
		go p.processFarmConsensus(farmID)
	}
}

// processFarmConsensus handles the consensus process for a specific farm
func (p *DexponentProtocol) processFarmConsensus(farmID int64) {
	// Lock the farm consensus map
	p.farmConsensusLock.Lock()

	// Get the farm state, create it if it doesn't exist
	farmState, exists := p.farmConsensus[farmID]
	if !exists {
		farmState = &FarmConsensusState{
			FarmID:          farmID,
			CurrentRound:    0,
			IsLeader:        false,
			RoundActive:     false,
			Scores:          make(map[peer.ID]float64),
			Benchmarks:      make(map[peer.ID]float64),
			Participants:    make(map[peer.ID]bool),
			CooldownEndTime: time.Time{},
		}
		p.farmConsensus[farmID] = farmState
	}

	// Check if a round is already active
	if farmState.RoundActive {
		// Check if the round has ended
		if time.Now().After(farmState.RoundEndTime) {
			// Finalize the round
			p.farmConsensusLock.Unlock()
			p.finalizeFarmConsensusRound(farmID)
			return
		}

		// Round is still active
		p.farmConsensusLock.Unlock()
		return
	}

	// Check cooldown period
	if !farmState.CooldownEndTime.IsZero() && time.Now().Before(farmState.CooldownEndTime) {
		// Only log cooldown message once per cooldown period
		if !farmState.CooldownLogged {
			fmt.Printf("Farm %d consensus in cooldown until %s (in %s)\n",
				farmID,
				farmState.CooldownEndTime.Format("15:04:05"),
				farmState.CooldownEndTime.Sub(time.Now()).Round(time.Second))
			// Mark that we've logged this cooldown period
			farmState.CooldownLogged = true
		}
		p.farmConsensusLock.Unlock()
		return
	}

	// If we're not in cooldown anymore, reset the cooldown logged flag
	farmState.CooldownLogged = false

	// We're not in a round and not in cooldown, check if we can start a new round
	peers := p.GetDexponentPeers()
	if len(peers) < 2 { // Need at least 3 peers including ourselves
		p.farmConsensusLock.Unlock()
		return
	}

	// Determine if we should be the leader
	p.farmConsensusLock.Unlock()
	p.startFarmConsensusRound(farmID)
}

// startFarmConsensusRound starts a new farm-specific consensus round
func (p *DexponentProtocol) startFarmConsensusRound(farmID int64) {
	// Lock the farm consensus map
	p.farmConsensusLock.Lock()
	farmState, exists := p.farmConsensus[farmID]
	if !exists {
		p.farmConsensusLock.Unlock()
		return
	}

	// If we have an Ethereum client, fetch the current round number from the contract
	if p.ethClient != nil {
		round, err := p.ethClient.GetCurrentFarmRound(farmID)
		if err != nil {
			fmt.Printf("⚠️ Failed to get current farm round from contract: %v\n", err)
		} else {
			fmt.Printf("ℹ️ Current farm %d round from contract: %d\n", farmID, round)

			// Always set our local round to match the contract round
			// This ensures we're always in sync with the contract
			farmState.CurrentRound = int64(round)
		}
	}

	// Select the leader based on the current round
	leaderID, ok := p.selectFarmLeader(farmID)
	if !ok {
		p.farmConsensusLock.Unlock()
		return
	}

	// Update the farm state
	farmState.CurrentRound++
	farmState.CurrentLeader = leaderID
	farmState.IsLeader = (leaderID == p.host.ID())
	farmState.RoundActive = true
	farmState.RoundStartTime = time.Now()
	farmState.RoundEndTime = time.Now().Add(30 * time.Second)
	farmState.Scores = make(map[peer.ID]float64)
	farmState.Benchmarks = make(map[peer.ID]float64)
	farmState.Participants = make(map[peer.ID]bool)

	// Generate farm returns
	farmState.FarmReturns = generateFarmReturns(20)

	// Log the start of the round
	fmt.Printf("🚀 Starting farm %d consensus round %d as %s\n",
		farmID, farmState.CurrentRound,
		map[bool]string{true: "leader", false: "participant"}[farmState.IsLeader])

	// If we're the leader, register with the contract and broadcast the start message
	if farmState.IsLeader {
		// Register as the farm leader in the contract if we have an Ethereum client
		if p.ethClient != nil {
			fmt.Printf("📝 Registering as leader for farm %d consensus round %d\n", farmID, farmState.CurrentRound)
			tx, err := p.ethClient.RegisterFarmLeader(farmID)
			if err != nil {
				fmt.Printf("❌ Failed to register as farm leader: %v\n", err)
			} else if tx != nil {
				// Only try to access tx.Hash() if tx is not nil
				fmt.Printf("✅ Successfully registered as farm leader, tx: %s\n", tx.Hash().Hex())
			} else {
				// If tx is nil but no error, we're already the leader
				fmt.Printf("✅ Already registered as farm leader for farm %d\n", farmID)
			}
		}

		// Calculate our own score first
		farmState.Scores[p.host.ID()] = calculateFarmScore(farmState.FarmReturns)
		farmState.Benchmarks[p.host.ID()] = calculateBenchmarkScore()
		farmState.Participants[p.host.ID()] = true

		// Create the consensus start payload
		startPayload := ConsensusStartPayload{
			RoundNumber: farmState.CurrentRound,
			FarmID:      farmID,
			FarmReturns: farmState.FarmReturns,
			StartTime:   farmState.RoundStartTime.Unix(),
			EndTime:     farmState.RoundEndTime.Unix(),
		}

		// Broadcast the start message
		p.BroadcastMessage(MessageTypeConsensusStart, startPayload)

		// Schedule finalization
		go func() {
			time.Sleep(time.Until(farmState.RoundEndTime))
			p.finalizeFarmConsensusRound(farmID)
		}()
	}

	p.farmConsensusLock.Unlock()
}

// finalizeFarmConsensusRound finalizes a farm-specific consensus round
func (p *DexponentProtocol) finalizeFarmConsensusRound(farmID int64) {
	// Lock the farm consensus map
	p.farmConsensusLock.Lock()

	// Get the farm state
	farmState, exists := p.farmConsensus[farmID]
	if !exists || !farmState.RoundActive {
		p.farmConsensusLock.Unlock()
		return
	}

	// Only the leader can finalize the round
	if !farmState.IsLeader {
		p.farmConsensusLock.Unlock()
		return
	}

	// Calculate the median score
	scoreValues := make([]float64, 0, len(farmState.Scores))
	for _, score := range farmState.Scores {
		scoreValues = append(scoreValues, score)
	}

	// Calculate the median benchmark
	benchmarkValues := make([]float64, 0, len(farmState.Benchmarks))
	for _, benchmark := range farmState.Benchmarks {
		benchmarkValues = append(benchmarkValues, benchmark)
	}

	// Calculate the final score and benchmark
	finalFarmScore := calculateMedian(scoreValues)
	finalBenchmark := calculateMedian(benchmarkValues)

	// Get the list of participants
	participants := make([]string, 0, len(farmState.Participants))
	for peerID := range farmState.Participants {
		participants = append(participants, peerID.String())
	}

	// Set the cooldown end time (45 seconds from now)
	farmState.RoundActive = false
	farmState.CooldownEndTime = time.Now().Add(60 * time.Second)

	// Create the result payload
	resultPayload := ConsensusResultPayload{
		RoundNumber:    farmState.CurrentRound,
		FarmID:         farmID,
		FinalScore:     finalFarmScore,
		FinalBenchmark: finalBenchmark,
		Participants:   participants,
		NextRoundStart: farmState.CooldownEndTime.Unix(),
	}

	// Broadcast the result
	p.BroadcastMessage(MessageTypeConsensusResult, resultPayload)

	// Log the result
	fmt.Printf("✅ Finalized farm %d consensus round %d with %d participants. Score: %.4f, Benchmark: %.4f\n",
		farmID, farmState.CurrentRound, len(participants), finalFarmScore, finalBenchmark)

	// Submit the result to the blockchain
	if p.ethClient != nil {
		go func() {
			// Get previous score and benchmark from the blockchain if available
			prevScore, prevBenchmark := 0.0, 0.0
			if p.ethClient != nil {
				prevScoreRaw, err := p.ethClient.GetFarmScore(farmID)
				if err == nil && prevScoreRaw != nil {
					prevScore = float64(prevScoreRaw.Uint64()) / 1e18
				}
				prevBenchmarkRaw, err := p.ethClient.GetFarmBenchmark(farmID)
				if err == nil && prevBenchmarkRaw != nil {
					prevBenchmark = float64(prevBenchmarkRaw.Uint64()) / 100 // Convert basis points to percentage
				}
			}

			// Submit both farm score and benchmark to the blockchain
			fmt.Printf("🔗 Submitting farm %d consensus result to blockchain (score: %.4f → %.4f, benchmark: %.2f%% → %.2f%%)\n",
				farmID, prevScore, finalFarmScore, prevBenchmark, finalBenchmark)

			// First submit the farm score
			txHash, err := p.ethClient.SubmitConsensusResult(farmID, finalFarmScore, participants)
			if err != nil {
				fmt.Printf("❌ Error submitting farm score: %v\n", err)
				return
			}

			fmt.Printf("📝 Farm %d score submitted, tx: %s\n", farmID, txHash)

			// Wait for the score transaction to be mined
			scoreReceipt, err := p.ethClient.WaitForTransaction(txHash)
			if err != nil {
				fmt.Printf("❌ Error waiting for score transaction: %v\n", err)
				return
			}

			// Check if the score transaction was successful
			if scoreReceipt.Status == 1 {
				fmt.Printf("✅ Farm %d score confirmed on blockchain\n", farmID)
			} else {
				fmt.Printf("❌ Farm %d score transaction failed on blockchain\n", farmID)
				// Continue with benchmark submission even if score submission fails
			}

			// Now submit the benchmark
			// Convert benchmark to basis points (multiply by 100)
			benchmarkBasisPoints := new(big.Int)
			benchmarkFloat := big.NewFloat(finalBenchmark * 100) // Convert to basis points (10.5% -> 1050)
			benchmarkFloat.Int(benchmarkBasisPoints)

			// Submit the benchmark
			tx, err := p.ethClient.SetFarmBenchmarkSecure(farmID, benchmarkBasisPoints)
			if err != nil {
				fmt.Printf("❌ Error submitting farm benchmark: %v\n", err)
				return
			}

			fmt.Printf("📝 Farm %d benchmark submitted, tx: %s\n", farmID, tx.Hash().Hex())

			// Wait for the benchmark transaction to be mined
			benchmarkTxHash := tx.Hash().Hex()
			benchmarkReceipt, err := p.ethClient.WaitForTransaction(benchmarkTxHash)
			if err != nil {
				fmt.Printf("❌ Error waiting for benchmark transaction: %v\n", err)
				return
			}

			// Check if the benchmark transaction was successful
			if benchmarkReceipt.Status == 1 {
				fmt.Printf("✅ Farm %d benchmark confirmed on blockchain\n", farmID)
			} else {
				fmt.Printf("❌ Farm %d benchmark transaction failed on blockchain\n", farmID)
			}
		}()
	}

	p.farmConsensusLock.Unlock()
}

// selectFarmLeader selects a leader for a farm-specific consensus round
func (p *DexponentProtocol) selectFarmLeader(farmID int64) (peer.ID, bool) {
	// Get all peers including ourselves
	peers := p.GetDexponentPeers()
	peers = append(peers, p.host.ID())

	// Sort the peers by ID for deterministic selection
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].String() < peers[j].String()
	})

	// Get the farm state
	farmState, exists := p.farmConsensus[farmID]
	if !exists {
		return "", false
	}

	// Select the leader based on the current round
	leaderIndex := int(farmState.CurrentRound) % len(peers)
	return peers[leaderIndex], true
}

// handleFarmConsensusStart processes a farm-specific consensus start message
func (p *DexponentProtocol) handleFarmConsensusStart(stream network.Stream, msg Message) {
	// Get the remote peer ID (the leader)
	remotePeer := stream.Conn().RemotePeer()

	// Extract the payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid payload type\n")
		stream.Reset()
		return
	}

	// Extract round number
	roundNumberFloat, ok := payload["round_number"].(float64)
	if !ok {
		fmt.Printf("Error: Missing round_number in payload\n")
		stream.Reset()
		return
	}
	roundNumber := int64(roundNumberFloat)

	// Extract farm ID
	farmIDFloat, ok := payload["farm_id"].(float64)
	if !ok {
		fmt.Printf("Error: Missing farm_id in payload\n")
		stream.Reset()
		return
	}
	farmID := int64(farmIDFloat)

	// Check if we're assigned to this farm
	if p.ethClient != nil {
		isActive, err := p.ethClient.IsVerifierActiveForFarm(farmID)
		if err != nil || !isActive {
			// We're not active for this farm, ignore the message
			stream.Reset()
			return
		}
	}

	// Extract farm returns
	farmReturnsInterface, ok := payload["farm_returns"].([]interface{})
	if !ok {
		fmt.Printf("Error: Missing farm_returns in payload\n")
		stream.Reset()
		return
	}

	// Convert farm returns to float64 slice
	farmReturns := make([]float64, len(farmReturnsInterface))
	for i, v := range farmReturnsInterface {
		farmReturnVal, ok := v.(float64)
		if !ok {
			fmt.Printf("Error: Invalid farm return value at index %d\n", i)
			stream.Reset()
			return
		}
		farmReturns[i] = farmReturnVal
	}

	// Extract start and end times
	startTimeFloat, ok := payload["start_time"].(float64)
	if !ok {
		fmt.Printf("Error: Missing start_time in payload\n")
		stream.Reset()
		return
	}
	startTime := time.Unix(int64(startTimeFloat), 0)

	endTimeFloat, ok := payload["end_time"].(float64)
	if !ok {
		fmt.Printf("Error: Missing end_time in payload\n")
		stream.Reset()
		return
	}
	endTime := time.Unix(int64(endTimeFloat), 0)

	// Update our farm consensus state
	p.farmConsensusLock.Lock()

	// Get or create the farm state
	farmState, exists := p.farmConsensus[farmID]
	if !exists {
		farmState = &FarmConsensusState{
			FarmID:         farmID,
			CurrentRound:   roundNumber,
			CurrentLeader:  remotePeer,
			IsLeader:       false,
			RoundActive:    true,
			RoundStartTime: startTime,
			RoundEndTime:   endTime,
			Scores:         make(map[peer.ID]float64),
			Benchmarks:     make(map[peer.ID]float64),
			Participants:   make(map[peer.ID]bool),
			FarmReturns:    farmReturns,
		}
		p.farmConsensus[farmID] = farmState
	} else {
		// Update existing farm state
		farmState.CurrentRound = roundNumber
		farmState.CurrentLeader = remotePeer
		farmState.IsLeader = false
		farmState.RoundActive = true
		farmState.RoundStartTime = startTime
		farmState.RoundEndTime = endTime
		farmState.FarmReturns = farmReturns
		farmState.Scores = make(map[peer.ID]float64)
		farmState.Benchmarks = make(map[peer.ID]float64)
		farmState.Participants = make(map[peer.ID]bool)
	}

	p.farmConsensusLock.Unlock()

	// Log receipt of consensus start
	fmt.Printf("🔄 Received consensus start for farm %d round %d. Calculating farm score & benchmark...\n", farmID, roundNumber)

	// Calculate our farm score and benchmark
	farmScore := calculateFarmScore(farmReturns)
	farmBenchmark := calculateBenchmarkScore()

	// Create our score submission payload
	scoreSubmissionPayload := ScoreSubmissionPayload{
		RoundNumber:   roundNumber,
		FarmID:        farmID,
		FarmScore:     farmScore,
		FarmBenchmark: farmBenchmark,
		SubmitterID:   p.host.ID().String(),
	}

	// Send our score to the leader
	fmt.Printf("📊 Submitting farm score %.4f and benchmark %.4f to leader for round %d\n",
		farmScore, farmBenchmark, roundNumber)
	p.SendMessageToPeer(remotePeer, MessageTypeScoreSubmission, scoreSubmissionPayload)

	// Close the stream
	if err := stream.Close(); err != nil {
		// Ignore "canceled" errors as they're expected during rapid stream open/close
		if !strings.Contains(err.Error(), "canceled") {
			fmt.Printf("Error closing stream after farm consensus start: %v\n", err)
		}
	}
}

// handleFarmScoreSubmission processes a farm-specific score submission
func (p *DexponentProtocol) handleFarmScoreSubmission(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// Extract the payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid payload type\n")
		stream.Reset()
		return
	}

	// Extract round number
	roundNumberFloat, ok := payload["round_number"].(float64)
	if !ok {
		fmt.Printf("Error: Missing round_number in payload\n")
		stream.Reset()
		return
	}
	roundNumber := int64(roundNumberFloat)

	// Extract farm ID
	farmIDFloat, ok := payload["farm_id"].(float64)
	if !ok {
		fmt.Printf("Error: Missing farm_id in payload\n")
		stream.Reset()
		return
	}
	farmID := int64(farmIDFloat)

	// Extract farm score
	farmScoreFloat, ok := payload["farm_score"].(float64)
	if !ok {
		fmt.Printf("Error: Missing farm_score in payload\n")
		stream.Reset()
		return
	}

	// Extract farm benchmark
	farmBenchmarkFloat, ok := payload["farm_benchmark"].(float64)
	if !ok {
		fmt.Printf("Error: Missing farm_benchmark in payload\n")
		stream.Reset()
		return
	}

	// Extract submitter ID
	submitterIDStr, ok := payload["submitter_id"].(string)
	if !ok {
		fmt.Printf("Error: Missing submitter_id in payload\n")
		stream.Reset()
		return
	}

	// Verify submitter ID matches remote peer
	submitterID, err := peer.Decode(submitterIDStr)
	if err != nil || submitterID != remotePeer {
		fmt.Printf("Error: Submitter ID mismatch or invalid\n")
		stream.Reset()
		return
	}

	// Update our farm consensus state
	p.farmConsensusLock.Lock()

	// Get the farm state
	farmState, exists := p.farmConsensus[farmID]
	if !exists || !farmState.RoundActive || farmState.CurrentRound != roundNumber {
		// Invalid farm state
		p.farmConsensusLock.Unlock()
		stream.Reset()
		return
	}

	// Only the leader should receive score submissions
	if !farmState.IsLeader {
		p.farmConsensusLock.Unlock()
		stream.Reset()
		return
	}

	// Store the score and benchmark
	farmState.Scores[remotePeer] = farmScoreFloat
	farmState.Benchmarks[remotePeer] = farmBenchmarkFloat
	farmState.Participants[remotePeer] = true

	// Log the submission
	fmt.Printf("📥 Received farm score %.4f & farm benchmark %.4f from %s for round %d (%d/%d submissions)\n",
		farmScoreFloat, farmBenchmarkFloat, remotePeer.String(), roundNumber,
		len(farmState.Scores), len(p.GetDexponentPeers())+1)

	p.farmConsensusLock.Unlock()

	// Close the stream
	if err := stream.Close(); err != nil {
		// Ignore "canceled" errors as they're expected during rapid stream open/close
		if !strings.Contains(err.Error(), "canceled") {
			fmt.Printf("Error closing stream after receiving farm score: %v\n", err)
		}
	}
}

// handleFarmConsensusResult processes a farm-specific consensus result message
func (p *DexponentProtocol) handleFarmConsensusResult(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// Extract the payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid payload type\n")
		stream.Reset()
		return
	}

	// Extract round number
	roundNumberFloat, ok := payload["round_number"].(float64)
	if !ok {
		fmt.Printf("Error: Missing round_number in payload\n")
		stream.Reset()
		return
	}
	roundNumber := int64(roundNumberFloat)

	// Extract farm ID
	farmIDFloat, ok := payload["farm_id"].(float64)
	if !ok {
		fmt.Printf("Error: Missing farm_id in payload\n")
		stream.Reset()
		return
	}
	farmID := int64(farmIDFloat)

	// Extract final score
	finalScoreFloat, ok := payload["final_score"].(float64)
	if !ok {
		fmt.Printf("Error: Missing final_score in payload\n")
		stream.Reset()
		return
	}

	// Extract final benchmark
	finalBenchmarkFloat, ok := payload["final_benchmark"].(float64)
	if !ok {
		fmt.Printf("Error: Missing final_benchmark in payload\n")
		stream.Reset()
		return
	}

	// Extract next round start time
	nextRoundStartFloat, ok := payload["next_round_start"].(float64)
	if !ok {
		fmt.Printf("Error: Missing next_round_start in payload\n")
		stream.Reset()
		return
	}

	// Update our farm consensus state
	p.farmConsensusLock.Lock()

	// Get the farm state
	farmState, exists := p.farmConsensus[farmID]
	if !exists {
		// Create a new farm state
		farmState = &FarmConsensusState{
			FarmID:       farmID,
			CurrentRound: roundNumber,
			RoundActive:  false,
			Scores:       make(map[peer.ID]float64),
			Benchmarks:   make(map[peer.ID]float64),
			Participants: make(map[peer.ID]bool),
		}
		p.farmConsensus[farmID] = farmState
	}

	// Update the farm state
	farmState.CurrentRound = roundNumber
	farmState.RoundActive = false
	farmState.CurrentLeader = remotePeer
	farmState.CooldownEndTime = time.Unix(int64(nextRoundStartFloat), 0)

	// Clear scores and benchmarks for the next round
	farmState.Scores = make(map[peer.ID]float64)
	farmState.Benchmarks = make(map[peer.ID]float64)
	farmState.Participants = make(map[peer.ID]bool)

	// Log the result
	nextRoundStart := time.Unix(int64(nextRoundStartFloat), 0)
	fmt.Printf("📋 Received farm %d consensus result for round %d: Score %.4f, Benchmark %.4f\n",
		farmID, roundNumber, finalScoreFloat, finalBenchmarkFloat)
	fmt.Printf("⏱️ Next farm %d consensus round will start at %s (in %s)\n",
		farmID,
		nextRoundStart.Format("15:04:05"),
		nextRoundStart.Sub(time.Now()).Round(time.Second))

	p.farmConsensusLock.Unlock()

	// Close the stream
	if err := stream.Close(); err != nil {
		// Ignore "canceled" errors as they're expected during rapid stream open/close
		if !strings.Contains(err.Error(), "canceled") {
			fmt.Printf("Error closing stream after receiving farm result: %v\n", err)
		}
	}
}
