package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p"
	coreconnmgr "github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	connmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// P2PHost wraps a libp2p host with context management and status reporting
type P2PHost struct {
	host       host.Host
	ctx        context.Context
	cancel     context.CancelFunc
	statusChan chan HostStatus
	mappedPorts []int // Track mapped ports for logging
}

// HostStatus reports the current NAT and connectivity status
type HostStatus struct {
	ExternalAddrs []multiaddr.Multiaddr
	Error         error
}

// Forward host.Host interface methods
func (ph *P2PHost) ID() peer.ID {
	return ph.host.ID()
}

func (ph *P2PHost) Peerstore() peerstore.Peerstore {
	return ph.host.Peerstore()
}

func (ph *P2PHost) Addrs() []multiaddr.Multiaddr {
	return ph.host.Addrs()
}

func (ph *P2PHost) Network() network.Network {
	return ph.host.Network()
}

func (ph *P2PHost) Mux() protocol.Switch {
	return ph.host.Mux()
}

func (ph *P2PHost) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return ph.host.Connect(ctx, pi)
}

func (ph *P2PHost) SetStreamHandler(pid protocol.ID, handler network.StreamHandler) {
	ph.host.SetStreamHandler(pid, handler)
}

func (ph *P2PHost) SetStreamHandlerMatch(pid protocol.ID, match func(protocol.ID) bool, handler network.StreamHandler) {
	ph.host.SetStreamHandlerMatch(pid, match, handler)
}

func (ph *P2PHost) RemoveStreamHandler(pid protocol.ID) {
	ph.host.RemoveStreamHandler(pid)
}

func (ph *P2PHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
	return ph.host.NewStream(ctx, p, pids...)
}

func (ph *P2PHost) ConnManager() coreconnmgr.ConnManager {
	return ph.host.ConnManager()
}

func (ph *P2PHost) EventBus() event.Bus {
	return ph.host.EventBus()
}

// NewHost creates a new libp2p host with enhanced NAT traversal and transport options
func NewHost() (*P2PHost, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Generate a key pair for this host
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create a multiaddress for the host to listen on
	listenAddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create multiaddress: %w", err)
	}

	// Create a connection manager with tuned parameters
	connManager, err := connmgr.NewConnManager(
		100,                                // Low watermark
		400,                                // High watermark
		connmgr.WithGracePeriod(5*time.Minute), // Grace period for trimming
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	// Create a libp2p host with enhanced features
	h, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(priv),
		libp2p.Security(noise.ID, noise.New),  // Primary security protocol
		libp2p.Security(tls.ID, tls.New),      // Fallback for interoperability
		libp2p.Transport(tcp.NewTCPTransport), // TCP transport
		libp2p.Transport(quic.NewTransport),   // QUIC transport for UDP-based connectivity
		libp2p.NATPortMap(),                   // UPnP/NAT-PMP for port mapping
		libp2p.EnableNATService(),             // NAT discovery service
		libp2p.EnableHolePunching(),          // Enable UDP hole punching
		libp2p.ConnectionManager(connManager),
		libp2p.Ping(true), // Enable ping for detecting dead connections
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	ph := &P2PHost{
		host:       h,
		ctx:        ctx,
		cancel:     cancel,
		statusChan: make(chan HostStatus, 10),
		mappedPorts: make([]int, 0),
	}

	// Perform aggressive NAT mapping with retries
	go ph.setupPersistentNATMapping(ctx)

	// Start NAT status monitoring
	go ph.monitorNATStatus()

	// Set a custom protocol handler for validator communication
	h.SetStreamHandler("/dexponent/validator/1.0.0", func(s network.Stream) {
		// Placeholder for validator-specific logic (e.g., process consensus messages)
		fmt.Printf("Received validator stream from %s\n", s.Conn().RemotePeer())
		s.Close()
	})

	return ph, nil
}

// setupPersistentNATMapping attempts to create persistent NAT port mappings
// with multiple retries to improve connectivity through restrictive NATs
func (ph *P2PHost) setupPersistentNATMapping(ctx context.Context) {
	// Wait a moment for the host to initialize
	time.Sleep(1 * time.Second)
	
	// Get all listen addresses
	for _, addr := range ph.host.Network().ListenAddresses() {
		// Extract port information
		port, err := extractPortFromMultiaddr(addr)
		if err != nil {
			continue
		}
		
		// Try multiple times with increasing timeout
		for i := 0; i < 5; i++ {
			// Use the built-in NAT manager from libp2p
			// This is already configured with NATPortMap() option
			time.Sleep(2 * time.Second)
			
			// Check if we have external addresses after waiting
			addrs := ph.host.Addrs()
			externalAddrs := filterExternalAddrs(addrs)
			
			if len(externalAddrs) > 0 {
				// Found external addresses, port mapping successful
				ph.mappedPorts = append(ph.mappedPorts, port)
				fmt.Printf("✅ Successfully mapped port %d after attempt %d\n", port, i+1)
				break
			}
			
			// If this is the last attempt, log failure
			if i == 4 {
				fmt.Printf("⚠️ Failed to map port %d after multiple attempts\n", port)
			}
		}
	}
}

// extractPortFromMultiaddr extracts the port from a multiaddress
func extractPortFromMultiaddr(addr multiaddr.Multiaddr) (int, error) {
	// Convert to string and parse
	addrStr := addr.String()
	parts := strings.Split(addrStr, "/")
	
	// Look for tcp or udp component followed by port
	for i, part := range parts {
		if (part == "tcp" || part == "udp") && i+1 < len(parts) {
			port, err := strconv.Atoi(parts[i+1])
			return port, err
		}
	}
	
	return 0, fmt.Errorf("no port found in multiaddress")
}

// Close shuts down the host and its associated resources
func (ph *P2PHost) Close() error {
	// Cancel context to stop all goroutines
	ph.cancel()
	
	// Close status channel
	close(ph.statusChan)
	
	// Close the host - this will also close NAT mappings
	return ph.host.Close()
}

// Status returns a channel for receiving host status updates
func (ph *P2PHost) Status() <-chan HostStatus {
	return ph.statusChan
}

// monitorNATStatus periodically checks and reports the NAT status
func (ph *P2PHost) monitorNATStatus() {
	// Log initial status
	ph.logInitialNATStatus()

	// Wait a bit for NAT detection to complete
	time.Sleep(5 * time.Second)

	// Check for NAT type using autonat service
	addrs := ph.host.Addrs()
	externalAddrs := filterExternalAddrs(addrs)
	
	// Try to determine NAT type based on behavior
	if len(addrs) > 0 {
		if len(externalAddrs) == 0 {
			// No external addresses suggests restrictive NAT
			fmt.Printf("⚠️ Warning: Restrictive NAT detected (likely symmetric NAT)\n")
			
			// Try more aggressive NAT traversal
			go ph.attemptAdditionalNATTraversal()
		} else {
			// We have external addresses, likely a cone NAT
			fmt.Printf("ℹ️ NAT type: Cone NAT (allows inbound connections via port mapping)\n")
			fmt.Printf("ℹ️ External addresses detected:\n")
			for _, addr := range externalAddrs {
				fmt.Printf("  %s/p2p/%s\n", addr, ph.ID().String())
			}
		}
	} else {
		fmt.Printf("⚠️ Warning: Unable to determine NAT status\n")
	}

	// Periodically check and report status
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Get external addresses
			externalAddrs := filterExternalAddrs(ph.host.Addrs())
			
			// Report status
			ph.statusChan <- HostStatus{
				ExternalAddrs: externalAddrs,
				Error:         nil,
			}
			
			// Log if no external addresses
			if len(externalAddrs) == 0 {
				fmt.Println("No external addresses detected. Using relays.")
			}
		case <-ph.ctx.Done():
			return
		}
	}
}

// attemptAdditionalNATTraversal tries alternative methods to traverse NAT
func (ph *P2PHost) attemptAdditionalNATTraversal() {
	// Try alternative port ranges
	for port := 10000; port < 10010; port++ {
		// Create a new listen address with a specific port
		addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
		if err != nil {
			continue
		}
		
		// Try to listen on this address
		if err := ph.host.Network().Listen(addr); err != nil {
			continue
		}
		
		fmt.Printf("✅ Successfully listening on alternative port %d\n", port)
		
		// Wait to see if we get external addresses
		time.Sleep(5 * time.Second)
		
		// Check if we have external addresses
		addrs := ph.host.Addrs()
		externalAddrs := filterExternalAddrs(addrs)
		
		if len(externalAddrs) > 0 {
			fmt.Printf("✅ Successfully obtained external address with alternative port %d\n", port)
			ph.mappedPorts = append(ph.mappedPorts, port)
			
			// Display the external addresses
			fmt.Printf("ℹ️ External addresses detected:\n")
			for _, addr := range externalAddrs {
				fmt.Printf("  %s/p2p/%s\n", addr, ph.ID().String())
			}
			
			return
		}
	}
}

// logInitialNATStatus logs and reports the initial NAT status
func (ph *P2PHost) logInitialNATStatus() {
	addrs := ph.host.Addrs()
	externalAddrs := filterExternalAddrs(addrs)
	ph.statusChan <- HostStatus{ExternalAddrs: externalAddrs}
	if len(externalAddrs) > 0 {
		fmt.Println("External addresses detected:")
		for _, addr := range externalAddrs {
			fmt.Printf("  %s/p2p/%s\n", addr, ph.ID().String())
		}
	} else {
		fmt.Println("No external addresses detected. Using relays.")
	}
}

// filterExternalAddrs returns only external (non-local) addresses
func filterExternalAddrs(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
	var external []multiaddr.Multiaddr
	for _, addr := range addrs {
		if !isLocalAddress(addr) {
			external = append(external, addr)
		}
	}
	return external
}

// isLocalAddress checks if a multiaddress is a local address
func isLocalAddress(addr multiaddr.Multiaddr) bool {
	// Extract the IP address from the multiaddr
	ip, err := manet.ToIP(addr)
	if err != nil {
		return true // If we can't parse it, assume it's local to be safe
	}

	// Check for loopback addresses
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local addresses
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for private network addresses
	if ip4 := ip.To4(); ip4 != nil {
		// Check for private IPv4 ranges
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// 169.254.0.0/16 (link local)
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 192.0.0.0/24 (IETF Protocol Assignments)
		if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0 {
			return true
		}
	} else {
		// Check for private IPv6 ranges
		// fc00::/7 (unique local addresses)
		if ip[0] == 0xfc || ip[0] == 0xfd {
			return true
		}
		// fe80::/10 (link-local addresses)
		if ip[0] == 0xfe && (ip[1] & 0xc0) == 0x80 {
			return true
		}
	}

	return false
}
