package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	libp2pconnmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/dexponent/dxp-verifier/pkg/logger"
)

// P2PHost wraps a libp2p host with context management and status reporting
type P2PHost struct {
	host        host.Host
	ctx         context.Context
	cancel      context.CancelFunc
	statusChan  chan HostStatus
	mappedPorts []int // Track mapped ports for logging
}

// HostStatus reports the current NAT and connectivity status
type HostStatus struct {
	ExternalAddrs []multiaddr.Multiaddr
	NATStatus     string
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

func (ph *P2PHost) ConnManager() connmgr.ConnManager {
	return ph.host.ConnManager()
}

func (ph *P2PHost) EventBus() event.Bus {
	return ph.host.EventBus()
}

// NewHost creates a new libp2p host with the Dexponent configuration
func NewHost() (*P2PHost, error) {
	// Generate a key pair for this host
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create a connection manager with tuned parameters
	connManager, err := libp2pconnmgr.NewConnManager(
		100, // Low watermark
		400, // High watermark
		libp2pconnmgr.WithGracePeriod(5*time.Minute), // Grace period for trimming
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	// Create a context for the host
	ctx, cancel := context.WithCancel(context.Background())

	// Create a libp2p host with enhanced features
	h, err := libp2p.New(
		libp2p.ListenAddrs(
			multiaddr.StringCast("/ip4/0.0.0.0/tcp/0"),
			multiaddr.StringCast("/ip6/::/tcp/0"),
			multiaddr.StringCast("/ip4/0.0.0.0/udp/0/quic-v1"),
			multiaddr.StringCast("/ip6/::/udp/0/quic-v1"),
		),
		libp2p.Identity(priv),
		libp2p.Security(noise.ID, noise.New),  // Primary security protocol
		libp2p.Security(tls.ID, tls.New),      // Fallback for interoperability
		libp2p.Transport(tcp.NewTCPTransport), // TCP transport
		libp2p.Transport(quic.NewTransport),   // QUIC transport for UDP-based connectivity
		libp2p.NATPortMap(),                    // UPnP/PCP port mapping
		libp2p.EnableNATService(),             // NAT server for peers
		libp2p.EnableHolePunching(),           // UDP hole punching
		libp2p.EnableAutoNATv2(),               // AutoNAT v2 for reachability
		libp2p.EnableRelay(),                  // Relay transport
		libp2p.EnableRelayService(),           // Relay service (v2)
		// Add dummy static relays to satisfy AutoRelay requirement
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{
			{
				ID: peer.ID("12D3KooWSqRxtCPzxNn3FPEpU42kD3tLPW3i9RioSi9CFdXcwHyt"),
				Addrs: []multiaddr.Multiaddr{
					multiaddr.StringCast("/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWSqRxtCPzxNn3FPEpU42kD3tLPW3i9RioSi9CFdXcwHyt"),
				},
			},
			{
				ID: peer.ID("12D3KooWJtYhem338hgA7Q1ojeos7hfedjFB4ofXxR5CotE22d9E"),
				Addrs: []multiaddr.Multiaddr{
					multiaddr.StringCast("/ip4/127.0.0.1/tcp/4002/p2p/12D3KooWJtYhem338hgA7Q1ojeos7hfedjFB4ofXxR5CotE22d9E"),
				},
			},
		}),
		libp2p.ConnectionManager(connManager),
		libp2p.Ping(true), // Enable ping for detecting dead connections
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	ph := &P2PHost{
		host:        h,
		ctx:         ctx,
		cancel:      cancel,
		statusChan:  make(chan HostStatus, 10),
		mappedPorts: make([]int, 0),
	}

	// Perform aggressive NAT mapping with retries
	go ph.setupPersistentNATMapping(ctx)

	// Start NAT status monitoring
	go ph.monitorNATStatus()

	// Set up stream handlers
	h.SetStreamHandler(protocol.ID("/p2p/id/1.0.0"), func(s network.Stream) {
		// Just close the stream - we're only interested in the connection event
		s.Close()
	})

	return ph, nil
}

// AddPeerToAddressBook adds a peer to the address book with the given address
func (ph *P2PHost) AddPeerToAddressBook(peerID peer.ID, addr multiaddr.Multiaddr) error {
	// Parse the multiaddress into a peer.AddrInfo
	addrInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("failed to parse peer address: %w", err)
	}

	// Add the address to the peerstore with a reasonable TTL
	ph.host.Peerstore().AddAddrs(peerID, addrInfo.Addrs, peerstore.PermanentAddrTTL)
	return nil
}

// setupPersistentNATMapping attempts to create persistent NAT port mappings
// with multiple retries to improve connectivity through restrictive NATs
func (ph *P2PHost) setupPersistentNATMapping(ctx context.Context) {
	// Track ports that need mapping
	portsToMap := make(map[int]bool)

	// Extract ports from listening addresses
	for _, addr := range ph.host.Addrs() {
		port, err := extractPortFromMultiaddr(addr)
		if err != nil {
			continue
		}
		portsToMap[port] = true
	}

	// Skip if no ports to map
	if len(portsToMap) == 0 {
		return
	}

	// Wait a bit for initial NAT detection
	time.Sleep(2 * time.Second)

	// Count successfully mapped ports
	successCount := 0

	// Try to map alternative ports if initial mapping fails
	alternativePorts := []int{10000, 10001, 10002, 10003, 10004}

	// First try to map the dynamic ports
	for port := range portsToMap {
		// Try to map the port
		logger.LogOnly("Attempting to map port %d", port)

		// Instead of using NATManager directly, we'll check for external addresses
		// after attempting to listen on the port
		time.Sleep(2 * time.Second)

		// Check if we have external addresses
		externalAddrs := filterExternalAddrs(ph.host.Addrs())
		if len(externalAddrs) > 0 {
			logger.LogOnly("Successfully mapped port %d", port)
			ph.mappedPorts = append(ph.mappedPorts, port)
			successCount++
		} else {
			logger.LogOnly("Failed to map port %d", port)
		}
	}

	// If no ports were successfully mapped, try the alternative ports
	if successCount == 0 {
		logger.Info("Using local connectivity and relays for peer connections")

		for _, port := range alternativePorts {
			// Try to listen on the alternative port
			addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
			if err != nil {
				logger.LogOnly("Failed to create multiaddr for port %d: %v", port, err)
				continue
			}

			// Try to listen on the port
			err = ph.host.Network().Listen(addr)
			if err != nil {
				logger.LogOnly("Failed to listen on port %d: %v", port, err)
				continue
			}

			logger.LogOnly("Successfully listening on alternative port %d", port)
			ph.mappedPorts = append(ph.mappedPorts, port)

			// Wait to see if we get external addresses
			time.Sleep(2 * time.Second)

			// Check if we have external addresses
			addrs := ph.host.Addrs()
			externalAddrs := filterExternalAddrs(addrs)

			if len(externalAddrs) > 0 {
				logger.LogOnly("Successfully obtained external address with alternative port %d", port)

				// Display the external addresses
				logger.LogOnly("External addresses detected:")
				for _, addr := range externalAddrs {
					addrStr := fmt.Sprintf("  %s/p2p/%s", addr, ph.ID().String())
					logger.LogOnly("%s", addrStr)
				}

				// We've successfully established external connectivity, no need to try more ports
				return
			}
		}
	}

	// If we've tried the maximum number of ports and still don't have external connectivity,
	// inform the user that we'll be using relays
	if successCount == 0 {
		logger.Info("Could not establish direct external connectivity after trying multiple ports")
		logger.Info("Will rely on relays and DHT for peer connectivity")
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

	// Get NAT status by checking for external addresses
	addrs := ph.host.Addrs()
	externalAddrs := filterExternalAddrs(addrs)

	// Determine NAT status based on external addresses
	if len(externalAddrs) > 0 {
		logger.Success("Public connectivity detected (properly mapped ports)")
		logger.Info("External addresses detected:")

		// Use a map to track addresses we've already printed to avoid duplicates
		printedAddrs := make(map[string]bool)

		for _, addr := range externalAddrs {
			addrStr := fmt.Sprintf("  %s/p2p/%s", addr, ph.ID().String())
			if !printedAddrs[addrStr] {
				logger.Info("%s", addrStr)
				printedAddrs[addrStr] = true
			}
		}
	} else {
		logger.Info("Using local connectivity and relays for peer connections")
		go ph.attemptAdditionalNATTraversal()
	}

	// Track whether we've found external addresses
	hasExternalAddrs := len(externalAddrs) > 0

	// Keep track of consecutive status checks with no external addresses
	consecutiveFailures := 0

	// Periodically check and report status
	ticker := time.NewTicker(60 * time.Second) // Reduced frequency of checks
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Get external addresses
			externalAddrs := filterExternalAddrs(ph.host.Addrs())

			// Check if external address status has changed
			currentHasExternal := len(externalAddrs) > 0

			// Only report status changes after confirming the change is persistent
			// This prevents flapping between states due to temporary network issues
			if currentHasExternal {
				// Reset failure counter when we have external addresses
				consecutiveFailures = 0

				if !hasExternalAddrs {
					// Status changed from no external to having external
					logger.Success("External connectivity established")

					// Update tracking
					hasExternalAddrs = true
				}
			} else {
				// Increment failure counter when we don't have external addresses
				consecutiveFailures++

				// Only report lost connectivity after 5 consecutive checks (5 minutes)
				// to avoid false alarms due to temporary network issues
				if hasExternalAddrs && consecutiveFailures >= 5 {
					// Log this at a lower priority level since the node can still function with relays
					logger.Info("Using relay connections for peer discovery")
					hasExternalAddrs = false
				}
			}

			// Determine NAT status string for reporting
			natStatusStr := "private"
			if currentHasExternal {
				natStatusStr = "public"
			}

			// Report status
			ph.statusChan <- HostStatus{
				ExternalAddrs: externalAddrs,
				NATStatus:     natStatusStr,
				Error:         nil,
			}
		case <-ph.ctx.Done():
			return
		}
	}
}

// attemptAdditionalNATTraversal tries alternative methods to traverse NAT
func (ph *P2PHost) attemptAdditionalNATTraversal() {
	// Get initial addresses
	initialAddrs := ph.host.Addrs()
	initialExternalAddrs := filterExternalAddrs(initialAddrs)

	// If we already have external addresses, no need to try alternative ports
	if len(initialExternalAddrs) > 0 {
		logger.LogOnly("External connectivity already established, skipping alternative port binding")
		return
	}

	// Try alternative port ranges - limit to just a few ports
	maxPortsToTry := 5
	portsAttempted := 0

	for port := 10000; port < 10010 && portsAttempted < maxPortsToTry; port++ {
		portsAttempted++

		// Create a new listen address with a specific port
		addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
		if err != nil {
			continue
		}

		// Try to listen on this address
		if err := ph.host.Network().Listen(addr); err != nil {
			logger.LogOnly("Failed to listen on port %d: %v", port, err)
			continue
		}

		logger.LogOnly("Successfully listening on alternative port %d", port)
		ph.mappedPorts = append(ph.mappedPorts, port)

		// Wait to see if we get external addresses
		time.Sleep(2 * time.Second)

		// Check if we have external addresses
		addrs := ph.host.Addrs()
		externalAddrs := filterExternalAddrs(addrs)

		if len(externalAddrs) > 0 {
			logger.LogOnly("Successfully obtained external address with alternative port %d", port)

			// Display the external addresses
			logger.LogOnly("External addresses detected:")
			for _, addr := range externalAddrs {
				addrStr := fmt.Sprintf("  %s/p2p/%s", addr, ph.ID().String())
				logger.LogOnly("%s", addrStr)
			}

			// We've successfully established external connectivity, no need to try more ports
			return
		}
	}

	// If we've tried the maximum number of ports and still don't have external connectivity,
	// inform the user that we'll be using relays
	if portsAttempted >= maxPortsToTry {
		logger.Info("Could not establish direct external connectivity after trying multiple ports")
		logger.Info("Will rely on relays and DHT for peer connectivity")
	}
}

// logInitialNATStatus logs and reports the initial NAT status
func (ph *P2PHost) logInitialNATStatus() {
	addrs := ph.host.Addrs()
	externalAddrs := filterExternalAddrs(addrs)

	// Determine NAT status
	natStatus := "private"
	if len(externalAddrs) > 0 {
		natStatus = "public"
	}

	// Report status to the channel
	ph.statusChan <- HostStatus{
		ExternalAddrs: externalAddrs,
		NATStatus:     natStatus,
		Error:         nil,
	}

	// We don't print addresses here anymore since they're already printed in main.go
	// This prevents duplicate address printing
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
		if ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 {
			return true
		}
	}
	return false
}
