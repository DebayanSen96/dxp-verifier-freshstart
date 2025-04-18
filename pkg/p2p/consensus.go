package p2p

import (
	"fmt"
	"math"
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
		var variance float64
		for _, r := range returns {
			variance += math.Pow(r-normalizedYield, 2)
		}
		variance /= float64(len(returns))
		// Higher consistency (lower variance) gives higher factor
		consistencyFactor = 1.0 / (1.0 + variance)
	}

	// Calculate final score
	farmScore := (normalizedYield * volumeWeight) * sortinoRatio * consistencyFactor

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
	}

	// Broadcast consensus start message
	fmt.Printf("🚀 Starting consensus round %d as leader. Round will end in %v.\n", p.currentRound, roundDuration)
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

	// Verify this is from the current leader
	if remotePeer != p.currentLeader {
		// Silently ignore messages from non-leaders
		stream.Reset()
		return
	}

	// Parse the payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid consensus start payload format\n")
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
		FarmScore:     farmScore,
		FarmBenchmark: farmBenchmark, // Include benchmark
		SubmitterID:   p.host.ID().String(),
	}

	// Send our score to the leader
	fmt.Printf("📊 Submitting farm score %.4f and benchmark %.4f to leader for round %d\n", farmScore, farmBenchmark, roundNumber)
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
	// Check if we are the leader
	if !p.isLeader {
		// Silently ignore score submissions if we're not the leader
		stream.Reset()
		return
	}

	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// Parse the payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		fmt.Printf("Error: Invalid score submission payload format\n")
		stream.Reset()
		return
	}

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

	fmt.Printf("📥 Received farm score %.4f & farm benchmark %.4f from %s for round %d (%d/%d submissions)\n",
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
