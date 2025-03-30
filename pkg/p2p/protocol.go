package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const (
	// DexponentProtocolID is the protocol identifier for Dexponent verifiers
	DexponentProtocolID = protocol.ID("/dexponent/verifier/1.0.0")

	// MaxMessageSize is the maximum size of a message in bytes
	MaxMessageSize = 1024 * 1024 // 1MB
)

// MessageType represents the type of message being sent
type MessageType string

const (
	// MessageTypeHandshake is sent when a peer connects to identify as a Dexponent verifier
	MessageTypeHandshake MessageType = "handshake"

	// MessageTypePing is a simple ping message
	MessageTypePing MessageType = "ping"

	// MessageTypePong is a response to a ping
	MessageTypePong MessageType = "pong"

	// MessageTypeData is used to share data between Dexponent peers
	MessageTypeData MessageType = "data"

	// Consensus message types
	MessageTypeLeaderElection MessageType = "leader_election"
	MessageTypeConsensusStart MessageType = "consensus_start"
	MessageTypeScoreSubmission MessageType = "score_submission"
	MessageTypeConsensusResult MessageType = "consensus_result"
)

// Message represents a message exchanged between Dexponent verifiers
type Message struct {
	// Type is the type of message
	Type MessageType `json:"type"`

	// Payload is the message payload
	Payload interface{} `json:"payload"`

	// Timestamp is when the message was created
	Timestamp int64 `json:"timestamp"`
}

// HandshakePayload is the payload for a handshake message
type HandshakePayload struct {
	// Version is the version of the Dexponent verifier
	Version string `json:"version"`

	// NodeID is a unique identifier for this node
	NodeID string `json:"node_id"`
}

// DataPayload is the payload for a data message
type DataPayload struct {
	// Key is the identifier for this data
	Key string `json:"key"`

	// Value is the actual data being shared
	Value string `json:"value"`

	// SenderID is the ID of the peer that originated this data
	SenderID string `json:"sender_id"`
}

// LeaderElectionPayload is the payload for a leader election message
type LeaderElectionPayload struct {
	// LeaderID is the ID of the elected leader
	LeaderID string `json:"leader_id"`

	// RoundNumber is the current consensus round number
	RoundNumber int64 `json:"round_number"`
}

// ConsensusStartPayload is the payload for a consensus start message
type ConsensusStartPayload struct {
	// RoundNumber is the current consensus round number
	RoundNumber int64 `json:"round_number"`

	// FarmReturns is the array of farm returns to calculate scores for
	FarmReturns []float64 `json:"farm_returns"`

	// StartTime is when the consensus round started
	StartTime int64 `json:"start_time"`

	// EndTime is when the consensus round will end
	EndTime int64 `json:"end_time"`
}

// ScoreSubmissionPayload is the payload for a score submission message
type ScoreSubmissionPayload struct {
	// RoundNumber is the consensus round this score is for
	RoundNumber int64 `json:"round_number"`

	// FarmScore is the calculated farm score
	FarmScore float64 `json:"farm_score"`

	// SubmitterID is the ID of the peer that calculated this score
	SubmitterID string `json:"submitter_id"`
}

// ConsensusResultPayload is the payload for a consensus result message
type ConsensusResultPayload struct {
	// RoundNumber is the consensus round this result is for
	RoundNumber int64 `json:"round_number"`

	// FinalScore is the consensus farm score
	FinalScore float64 `json:"final_score"`

	// Participants is the list of peers that participated in this round
	Participants []string `json:"participants"`

	// NextRoundStart is when the next consensus round will start
	NextRoundStart int64 `json:"next_round_start"`
}

// DexponentProtocol manages the Dexponent protocol
type DexponentProtocol struct {
	host         host.Host
	dexPeers     map[peer.ID]bool
	dexPeersLock sync.RWMutex
	
	// Consensus related fields
	currentRound      int64
	isLeader         bool
	currentLeader    peer.ID
	roundActive      bool
	roundStartTime   time.Time
	roundEndTime     time.Time
	cooldownEndTime  time.Time
	
	// Farm returns and scores
	farmReturns      []float64
	scores           map[peer.ID]float64
	scoresLock       sync.RWMutex
	consensusResult  float64
}

// NewDexponentProtocol creates a new Dexponent protocol handler
func NewDexponentProtocol(h host.Host) *DexponentProtocol {
	p := &DexponentProtocol{
		host:         h,
		dexPeers:     make(map[peer.ID]bool),
		currentRound: 0,
		isLeader:     false,
		roundActive:  false,
		scores:       make(map[peer.ID]float64),
	}

	// Set the stream handler for the Dexponent protocol
	h.SetStreamHandler(DexponentProtocolID, p.handleStream)

	// Start a goroutine to periodically send pings to connected Dexponent peers
	go p.pingPeers()

	return p
}

// handleStream handles incoming streams from peers
func (p *DexponentProtocol) handleStream(stream network.Stream) {
	// Set a deadline for reading from the stream
	if err := stream.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		fmt.Printf("Error setting read deadline: %v\n", err)
		stream.Reset()
		return
	}

	// Create a new decoder for the stream
	decoder := json.NewDecoder(stream)

	// Decode the message
	var msg Message
	if err := decoder.Decode(&msg); err != nil {
		fmt.Printf("Error decoding message: %v\n", err)
		stream.Reset()
		return
	}

	// Reset the read deadline
	if err := stream.SetReadDeadline(time.Time{}); err != nil {
		fmt.Printf("Error resetting read deadline: %v\n", err)
		stream.Reset()
		return
	}

	// Handle the message based on its type
	switch msg.Type {
	case MessageTypeHandshake:
		p.handleHandshake(stream, msg)
	case MessageTypePing:
		p.handlePing(stream, msg)
	case MessageTypePong:
		p.handlePong(stream, msg)
	case MessageTypeData:
		p.handleData(stream, msg)
	// Consensus message handlers
	case MessageTypeLeaderElection:
		p.handleLeaderElection(stream, msg)
	case MessageTypeConsensusStart:
		p.handleConsensusStart(stream, msg)
	case MessageTypeScoreSubmission:
		p.handleScoreSubmission(stream, msg)
	case MessageTypeConsensusResult:
		p.handleConsensusResult(stream, msg)
	default:
		fmt.Printf("Unknown message type: %s\n", msg.Type)
		stream.Reset()
	}
}

// handleHandshake processes a handshake message
func (p *DexponentProtocol) handleHandshake(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// Extract the handshake payload
	var handshake HandshakePayload
	switch payload := msg.Payload.(type) {
	case map[string]interface{}:
		// JSON unmarshalling from another peer
		handshake = HandshakePayload{
			Version: payload["version"].(string),
			NodeID:  payload["node_id"].(string),
		}
	case HandshakePayload:
		// Direct struct from our own code
		handshake = payload
	default:
		fmt.Printf("Invalid handshake payload format: %T\n", msg.Payload)
		stream.Reset()
		return
	}

	// Log the handshake
	fmt.Printf("✅ Received handshake from Dexponent peer %s (version: %s)\n",
		remotePeer.String(), handshake.Version)

	// Add this peer to our list of Dexponent peers
	p.dexPeersLock.Lock()
	p.dexPeers[remotePeer] = true
	p.dexPeersLock.Unlock()

	// Send a handshake response
	response := Message{
		Type: MessageTypeHandshake,
		Payload: HandshakePayload{
			Version: "1.0.0", // Our version
			NodeID:  p.host.ID().String(),
		},
		Timestamp: time.Now().Unix(),
	}

	// Send the response
	if err := json.NewEncoder(stream).Encode(response); err != nil {
		fmt.Printf("Error sending handshake response: %v\n", err)
		stream.Reset()
		return
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}

// handlePing processes a ping message
func (p *DexponentProtocol) handlePing(stream network.Stream, msg Message) {
	// No longer logging pings to reduce console noise

	// Send a pong response
	response := Message{
		Type:      MessageTypePong,
		Payload:   nil,
		Timestamp: time.Now().Unix(),
	}

	// Send the response
	if err := json.NewEncoder(stream).Encode(response); err != nil {
		fmt.Printf("Error sending pong response: %v\n", err)
		stream.Reset()
		return
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}

// handlePong processes a pong message
func (p *DexponentProtocol) handlePong(stream network.Stream, msg Message) {
	// No longer logging pongs to reduce console noise

	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}

// handleData processes a data message
func (p *DexponentProtocol) handleData(stream network.Stream, msg Message) {
	// Get the remote peer ID
	remotePeer := stream.Conn().RemotePeer()

	// Extract the data payload
	var data DataPayload
	switch payload := msg.Payload.(type) {
	case map[string]interface{}:
		// JSON unmarshalling from another peer
		data = DataPayload{
			Key:      payload["key"].(string),
			Value:    payload["value"].(string),
			SenderID: payload["sender_id"].(string),
		}
	case DataPayload:
		// Direct struct from our own code
		data = payload
	default:
		fmt.Printf("Invalid data payload format: %T\n", msg.Payload)
		stream.Reset()
		return
	}

	// Log the received data
	fmt.Printf("📦 Received data from %s - Key: %s, Value: %s\n", 
		remotePeer.String(), data.Key, data.Value)

	// Close the stream
	if err := stream.Close(); err != nil {
		fmt.Printf("Error closing stream: %v\n", err)
	}
}

// BroadcastData sends a data message to all connected Dexponent peers
func (p *DexponentProtocol) BroadcastData(key, value string) error {
	// Create the data payload
	data := DataPayload{
		Key:      key,
		Value:    value,
		SenderID: p.host.ID().String(),
	}

	// Create the message
	msg := Message{
		Type:      MessageTypeData,
		Payload:   data,
		Timestamp: time.Now().Unix(),
	}

	// Get all Dexponent peers
	p.dexPeersLock.RLock()
	peers := make([]peer.ID, 0, len(p.dexPeers))
	for peerID := range p.dexPeers {
		peers = append(peers, peerID)
	}
	p.dexPeersLock.RUnlock()

	// Send the message to all Dexponent peers
	var lastErr error
	sentCount := 0
	for _, peerID := range peers {
		err := p.sendMessage(peerID, msg)
		if err != nil {
			lastErr = err
			// Silently record the error but don't log it
		} else {
			sentCount++
		}
	}

	fmt.Printf("📤 Broadcasted data (Key: %s) to %d/%d Dexponent peers\n", key, sentCount, len(peers))

	return lastErr
}

// sendMessage sends a message to a specific peer
func (p *DexponentProtocol) sendMessage(peerID peer.ID, msg Message) error {
	// Create a new stream
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := p.host.NewStream(ctx, peerID, DexponentProtocolID)
	if err != nil {
		return fmt.Errorf("error creating stream: %w", err)
	}

	// Send the message
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		stream.Reset()
		return fmt.Errorf("error sending message: %w", err)
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		return fmt.Errorf("error closing stream: %w", err)
	}

	return nil
}

// pingPeers periodically sends pings to connected Dexponent peers
func (p *DexponentProtocol) pingPeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.dexPeersLock.RLock()
			peers := make([]peer.ID, 0, len(p.dexPeers))
			for peerID := range p.dexPeers {
				peers = append(peers, peerID)
			}
			p.dexPeersLock.RUnlock()

			for _, peerID := range peers {
				go p.sendPing(peerID)
			}
		}
	}
}

// sendPing sends a ping message to a peer
func (p *DexponentProtocol) sendPing(peerID peer.ID) {
	// Create a new stream
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := p.host.NewStream(ctx, peerID, DexponentProtocolID)
	if err != nil {
		// Silently remove the peer from our list if we can't connect to it
		p.dexPeersLock.Lock()
		delete(p.dexPeers, peerID)
		p.dexPeersLock.Unlock()
		return
	}

	// Create a ping message
	msg := Message{
		Type:      MessageTypePing,
		Payload:   nil,
		Timestamp: time.Now().Unix(),
	}

	// Send the message
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		fmt.Printf("Error sending ping to %s: %v\n", peerID.String(), err)
		stream.Reset()
		return
	}

	// We don't need to wait for a response here, as it will be handled by the stream handler
}

// SendHandshake sends a handshake message to a peer
func (p *DexponentProtocol) SendHandshake(peerID peer.ID) error {
	// Skip if this is already a known Dexponent peer
	p.dexPeersLock.RLock()
	if _, ok := p.dexPeers[peerID]; ok {
		p.dexPeersLock.RUnlock()
		return nil // Already a known Dexponent peer
	}
	p.dexPeersLock.RUnlock()

	// Create a new stream
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := p.host.NewStream(ctx, peerID, DexponentProtocolID)
	if err != nil {
		// This is expected for non-Dexponent peers
		if strings.Contains(err.Error(), "failed to negotiate protocol") {
			// Silently return for peers that don't support our protocol
			return nil
		}
		return fmt.Errorf("error creating stream: %w", err)
	}

	// If we got here, we successfully created a stream with a peer that supports our protocol!
	fmt.Printf("✅ Successfully established Dexponent protocol connection with %s\n", peerID.String())

	// Create a handshake message
	msg := Message{
		Type: MessageTypeHandshake,
		Payload: HandshakePayload{
			Version: "1.0.0", // Our version
			NodeID:  p.host.ID().String(),
		},
		Timestamp: time.Now().Unix(),
	}

	// Send the message
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		stream.Reset()
		return fmt.Errorf("error sending handshake: %w", err)
	}

	// Add this peer to our list of Dexponent peers immediately
	p.dexPeersLock.Lock()
	p.dexPeers[peerID] = true
	p.dexPeersLock.Unlock()

	// Wait for response
	var response Message
	if err := json.NewDecoder(stream).Decode(&response); err != nil {
		// It's okay if we don't get a response, the stream handler will handle it
		fmt.Printf("Note: No handshake response from %s: %v\n", peerID.String(), err)
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		return fmt.Errorf("error closing stream: %w", err)
	}

	return nil
}

// GetDexponentPeers returns a list of peers that are running the Dexponent protocol
func (p *DexponentProtocol) GetDexponentPeers() []peer.ID {
	p.dexPeersLock.RLock()
	defer p.dexPeersLock.RUnlock()

	peers := make([]peer.ID, 0, len(p.dexPeers))
	for peerID := range p.dexPeers {
		peers = append(peers, peerID)
	}

	return peers
}

// IsDexponentPeer checks if a peer is running the Dexponent protocol
func (p *DexponentProtocol) IsDexponentPeer(peerID peer.ID) bool {
	p.dexPeersLock.RLock()
	defer p.dexPeersLock.RUnlock()

	_, ok := p.dexPeers[peerID]
	return ok
}

// SendMessageToPeer sends a custom message to a specific Dexponent peer
func (p *DexponentProtocol) SendMessageToPeer(peerID peer.ID, msgType MessageType, payload interface{}) error {
	// Check if this is a Dexponent peer
	if !p.IsDexponentPeer(peerID) {
		return fmt.Errorf("peer %s is not a Dexponent peer", peerID.String())
	}

	// Create a new stream
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := p.host.NewStream(ctx, peerID, DexponentProtocolID)
	if err != nil {
		return fmt.Errorf("error creating stream: %w", err)
	}

	// Create the message
	msg := Message{
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}

	// Send the message
	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		stream.Reset()
		return fmt.Errorf("error sending message: %w", err)
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		return fmt.Errorf("error closing stream: %w", err)
	}

	return nil
}

// BroadcastMessage sends a message to all connected Dexponent peers
func (p *DexponentProtocol) BroadcastMessage(msgType MessageType, payload interface{}) {
	peers := p.GetDexponentPeers()

	for _, peerID := range peers {
		go func(pid peer.ID) {
			// Silently ignore errors when broadcasting messages
			_ = p.SendMessageToPeer(pid, msgType, payload)
		}(peerID)
	}
}
