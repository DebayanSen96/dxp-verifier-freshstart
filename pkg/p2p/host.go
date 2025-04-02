package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
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
	}

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

// Close shuts down the host and its associated resources
func (ph *P2PHost) Close() error {
	ph.cancel()
	close(ph.statusChan)
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
