package p2p

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
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

// selectLeader deterministically selects a leader from the list of peers
func (p *DexponentProtocol) selectLeader() (peer.ID, bool) {
	peers := p.GetDexponentPeers()
	
	// Add our own ID to the list
	allPeers := append(peers, p.host.ID())
	
	// Need at least 3 peers for consensus
	if len(allPeers) < 3 {
		return "", false
	}
	
	// Sort peer IDs lexicographically to ensure everyone gets the same order
	sort.Slice(allPeers, func(i, j int) bool {
		return allPeers[i].String() < allPeers[j].String()
	})
	
	// Use round number to rotate leadership
	leaderIndex := p.currentRound % int64(len(allPeers))
	leader := allPeers[leaderIndex]
	
	// Check if we are the leader
	isLeader := leader == p.host.ID()
	
	return leader, isLeader
}

// StartConsensusProcess initiates the consensus process if enough peers are connected
func (p *DexponentProtocol) StartConsensusProcess() {
	// Check if we're already in an active round or cooldown period
	if p.roundActive || time.Now().Before(p.cooldownEndTime) {
		return
	}
	
	// Select a leader
	leader, isLeader := p.selectLeader()
	if !isLeader {
		// We're not the leader, just update our state
		p.isLeader = false
		p.currentLeader = leader
		return
	}
	
	// We are the leader for this round
	p.isLeader = true
	p.currentLeader = p.host.ID()
	p.currentRound++
	
	// Generate farm returns
	p.farmReturns = generateFarmReturns(20)
	
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
	// Set round timing
	startTime := time.Now()
	endTime := startTime.Add(15 * time.Second)
	
	// Update protocol state
	p.roundActive = true
	p.roundStartTime = startTime
	p.roundEndTime = endTime
	p.scoresLock.Lock()
	p.scores = make(map[peer.ID]float64)
	// Add our own score
	p.scores[p.host.ID()] = calculateFarmScore(p.farmReturns)
	p.scoresLock.Unlock()
	
	// Create consensus start payload
	consensusStartPayload := ConsensusStartPayload{
		RoundNumber: p.currentRound,
		FarmReturns: p.farmReturns,
		StartTime:   startTime.Unix(),
		EndTime:     endTime.Unix(),
	}
	
	// Broadcast consensus start message
	fmt.Printf("🚀 Starting consensus round %d as leader. Round will end in 15 seconds.\n", p.currentRound)
	p.BroadcastMessage(MessageTypeConsensusStart, consensusStartPayload)
	
	// Schedule the end of the round
	time.AfterFunc(15*time.Second, func() {
		p.finalizeConsensusRound()
	})
}

// finalizeConsensusRound finalizes the consensus round and broadcasts results
func (p *DexponentProtocol) finalizeConsensusRound() {
	// Only the leader should finalize the round
	if !p.isLeader || !p.roundActive {
		return
	}
	
	p.roundActive = false
	p.cooldownEndTime = time.Now().Add(10 * time.Second)
	
	// Collect all scores
	p.scoresLock.RLock()
	scores := p.scores
	p.scoresLock.RUnlock()
	
	// Check if we have enough scores for consensus (at least 2/3 of peers)
	peers := p.GetDexponentPeers()
	allPeers := append(peers, p.host.ID())
	requiredScores := (len(allPeers) * 2) / 3
	if len(scores) < requiredScores {
		fmt.Printf("⚠️ Not enough scores for consensus. Got %d, need %d\n", len(scores), requiredScores)
		return
	}
	
	// Calculate consensus score (median of all scores)
	var scoreValues []float64
	for _, score := range scores {
		scoreValues = append(scoreValues, score)
	}
	sort.Float64s(scoreValues)
	
	var consensusScore float64
	if len(scoreValues) % 2 == 0 {
		// Even number of scores, take average of middle two
		middle := len(scoreValues) / 2
		consensusScore = (scoreValues[middle-1] + scoreValues[middle]) / 2
	} else {
		// Odd number of scores, take middle one
		middle := len(scoreValues) / 2
		consensusScore = scoreValues[middle]
	}
	
	// Round to 4 decimal places
	consensusScore = math.Round(consensusScore*10000) / 10000
	p.consensusResult = consensusScore
	
	// Create list of participants
	participants := make([]string, 0, len(scores))
	for peerID := range scores {
		participants = append(participants, peerID.String())
	}
	
	// Create consensus result payload
	resultPayload := ConsensusResultPayload{
		RoundNumber:    p.currentRound,
		FinalScore:     consensusScore,
		Participants:   participants,
		NextRoundStart: p.cooldownEndTime.Unix(),
	}
	
	// Broadcast consensus result
	fmt.Printf("✅ Consensus round %d complete. Final score: %.4f with %d participants\n", 
		p.currentRound, consensusScore, len(participants))
	p.BroadcastMessage(MessageTypeConsensusResult, resultPayload)
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
	
	// Update our state
	p.currentRound = roundNumber
	p.currentLeader = leaderID
	p.isLeader = (leaderID == p.host.ID())
	
	fmt.Printf("📢 Received leader election for round %d. Leader: %s\n", roundNumber, leaderIDStr)
	
	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}

// handleConsensusStart processes a consensus start message
func (p *DexponentProtocol) handleConsensusStart(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()
	
	// Verify this is from the current leader
	if remotePeer != p.currentLeader {
		fmt.Printf("Warning: Received consensus start from non-leader peer %s\n", remotePeer.String())
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
	
	fmt.Printf("🔄 Received consensus start for round %d. Calculating farm score...\n", roundNumber)
	
	// Calculate our farm score
	farmScore := calculateFarmScore(farmReturns)
	
	// Create score submission payload
	scorePayload := ScoreSubmissionPayload{
		RoundNumber: roundNumber,
		FarmScore:   farmScore,
		SubmitterID: p.host.ID().String(),
	}
	
	// Send our score to the leader
	fmt.Printf("📊 Submitting farm score %.4f to leader for round %d\n", farmScore, roundNumber)
	p.SendMessageToPeer(p.currentLeader, MessageTypeScoreSubmission, scorePayload)
	
	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}

// handleScoreSubmission processes a score submission message
func (p *DexponentProtocol) handleScoreSubmission(stream network.Stream, msg Message) {
	// Only the leader should process score submissions
	if !p.isLeader || !p.roundActive {
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
		stream.Reset()
		return
	}
	roundNumber := int64(roundNumberFloat)
	
	// Verify this is for the current round
	if roundNumber != p.currentRound {
		fmt.Printf("Warning: Received score for wrong round %d (current: %d)\n", roundNumber, p.currentRound)
		stream.Reset()
		return
	}
	
	farmScoreFloat, ok := payload["farm_score"].(float64)
	if !ok {
		fmt.Printf("Error: Missing farm_score in payload\n")
		stream.Reset()
		return
	}
	
	// Add the score to our collection
	p.scoresLock.Lock()
	p.scores[remotePeer] = farmScoreFloat
	scoreCount := len(p.scores)
	p.scoresLock.Unlock()
	
	fmt.Printf("📥 Received farm score %.4f from %s for round %d (%d/%d scores)\n", 
		farmScoreFloat, remotePeer.String(), roundNumber, scoreCount, len(p.GetDexponentPeers())+1)
	
	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}

// handleConsensusResult processes a consensus result message
func (p *DexponentProtocol) handleConsensusResult(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()
	
	// Verify this is from the current leader
	if remotePeer != p.currentLeader {
		fmt.Printf("Warning: Received consensus result from non-leader peer %s\n", remotePeer.String())
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
		stream.Reset()
		return
	}
	roundNumber := int64(roundNumberFloat)
	
	finalScoreFloat, ok := payload["final_score"].(float64)
	if !ok {
		fmt.Printf("Error: Missing final_score in payload\n")
		stream.Reset()
		return
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
			fmt.Printf("Error: Invalid participant value at index %d\n", i)
			stream.Reset()
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
	p.cooldownEndTime = time.Unix(int64(nextRoundStartFloat), 0)
	
	fmt.Printf("✅ Consensus round %d result received. Final score: %.4f with %d participants\n", 
		roundNumber, finalScoreFloat, len(participants))
	fmt.Printf("⏱️ Next consensus round will start after %s\n", 
		time.Unix(int64(nextRoundStartFloat), 0).Format(time.RFC3339))
	
	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}
