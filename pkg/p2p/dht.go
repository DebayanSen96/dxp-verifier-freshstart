package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// DefaultBootstrapPeers is a list of public DHT bootstrap nodes
var DefaultBootstrapPeers = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
}

// DHTService manages the DHT for peer discovery
type DHTService struct {
	host           host.Host
	dht            *dual.DHT
	peerStore      map[peer.ID]*PeerInfo
	peerStoreMutex sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	errChan        chan error
	wg             sync.WaitGroup
}

// PeerInfo stores information about a peer
type PeerInfo struct {
	ID              peer.ID
	LastSeen        time.Time
	IsReachable     bool
	ProtocolSupport []protocol.ID
	ConnectionType  string // "direct", "relay", "nat-traversal"
}

// NewDHT creates a new DHT service
func NewDHT(h host.Host) (*DHTService, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a dual DHT (LAN + WAN)
	kadDHT, err := dual.New(ctx, h,
		dual.DHTOption(dht.Mode(dht.ModeServer)),
		dual.DHTOption(dht.BootstrapPeers(getBootstrapPeers()...)),
		dual.DHTOption(dht.ProtocolPrefix("/dexponent")),
		dual.DHTOption(dht.BucketSize(20)),
		dual.DHTOption(dht.MaxRecordAge(24*time.Hour)),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	d := &DHTService{
		host:      h,
		dht:       kadDHT,
		peerStore: make(map[peer.ID]*PeerInfo),
		ctx:       ctx,
		cancel:    cancel,
		errChan:   make(chan error, 10),
	}

	// Start background routines
	d.wg.Add(3)
	go d.findPeersRoutine()
	go d.maintainDHT()
	go d.checkPeerHealth()

	return d, nil
}

// Bootstrap initiates DHT bootstrapping
func (d *DHTService) Bootstrap() error {
	bootstrapPeers := getBootstrapPeers()
	connected := false
	for _, peerInfo := range bootstrapPeers {
		if err := d.host.Connect(d.ctx, peerInfo); err == nil {
			connected = true
			fmt.Printf("Connected to bootstrap peer: %s\n", peerInfo.ID)
		} else {
			fmt.Printf("Failed to connect to bootstrap peer %s: %v\n", peerInfo.ID, err)
		}
	}
	if !connected {
		return fmt.Errorf("failed to connect to any bootstrap peers")
	}

	fmt.Println("Bootstrapping the DHT...")
	return d.dht.Bootstrap(d.ctx)
}

// findPeersRoutine periodically discovers new peers
func (d *DHTService) findPeersRoutine() {
	defer d.wg.Done()
	if err := d.discoverPeers(); err != nil {
		d.errChan <- fmt.Errorf("initial peer discovery failed: %w", err)
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			if err := d.discoverPeers(); err != nil {
				d.errChan <- fmt.Errorf("peer discovery failed: %w", err)
			}
		}
	}
}

// discoverPeers finds and connects to peers via the DHT
func (d *DHTService) discoverPeers() error {
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	// Get peers from the routing table instead of using GetClosestPeers
	var routingTablePeers []peer.ID
	
	// Get peers from both LAN and WAN DHTs
	if d.dht.LAN != nil {
		lanPeers := d.dht.LAN.RoutingTable().ListPeers()
		routingTablePeers = append(routingTablePeers, lanPeers...)
	}
	
	if d.dht.WAN != nil {
		wanPeers := d.dht.WAN.RoutingTable().ListPeers()
		routingTablePeers = append(routingTablePeers, wanPeers...)
	}
	
	// Deduplicate peers
	peerMap := make(map[peer.ID]bool)
	var uniquePeers []peer.ID
	
	for _, p := range routingTablePeers {
		if !peerMap[p] {
			peerMap[p] = true
			uniquePeers = append(uniquePeers, p)
		}
	}

	newPeers := make(map[peer.ID]*PeerInfo)
	for _, p := range uniquePeers {
		if p == d.host.ID() || d.host.Network().Connectedness(p) == network.Connected {
			continue
		}
		addrInfo := d.host.Peerstore().PeerInfo(p)
		if err := d.host.Connect(ctx, addrInfo); err == nil {
			// Get protocols safely
			protocols, _ := d.host.Peerstore().GetProtocols(p)
			
			newPeers[p] = &PeerInfo{
				ID:              p,
				LastSeen:        time.Now(),
				IsReachable:     true,
				ConnectionType:  "direct", // Update based on relay/hole-punching if applicable
				ProtocolSupport: protocols,
			}
			fmt.Printf("Connected to peer: %s\n", p)
		}
	}

	d.peerStoreMutex.Lock()
	for id, info := range newPeers {
		d.peerStore[id] = info
	}
	d.peerStoreMutex.Unlock()

	fmt.Printf("Discovered %d new peers. Total connected: %d\n", len(newPeers), len(d.host.Network().Peers()))
	return nil
}

// maintainDHT refreshes the DHT routing table
func (d *DHTService) maintainDHT() {
	defer d.wg.Done()
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			fmt.Println("Refreshing DHT routing table...")
			if err := d.dht.Bootstrap(d.ctx); err != nil {
				d.errChan <- fmt.Errorf("failed to refresh DHT: %w", err)
			}
			d.cleanupStalePeers()
		}
	}
}

// checkPeerHealth verifies peer reachability
func (d *DHTService) checkPeerHealth() {
	defer d.wg.Done()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.peerStoreMutex.Lock()
			for id, info := range d.peerStore {
				if time.Since(info.LastSeen) < 30*time.Minute {
					continue
				}
				ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
				if d.host.Network().Connectedness(id) != network.Connected {
					if err := d.host.Connect(ctx, peer.AddrInfo{ID: id, Addrs: d.host.Peerstore().Addrs(id)}); err != nil {
						info.IsReachable = false
						fmt.Printf("Peer %s is unreachable\n", id)
					} else {
						info.LastSeen = time.Now()
						info.IsReachable = true
					}
				} else {
					info.LastSeen = time.Now()
				}
				cancel()
			}
			d.peerStoreMutex.Unlock()
		}
	}
}

// cleanupStalePeers removes outdated peers
func (d *DHTService) cleanupStalePeers() {
	d.peerStoreMutex.Lock()
	defer d.peerStoreMutex.Unlock()

	staleThreshold := time.Now().Add(-3 * time.Hour)
	var stalePeers []peer.ID
	for id, info := range d.peerStore {
		if !info.IsReachable && info.LastSeen.Before(staleThreshold) {
			stalePeers = append(stalePeers, id)
		}
	}

	for _, id := range stalePeers {
		delete(d.peerStore, id)
		fmt.Printf("Removed stale peer %s\n", id)
	}
	if len(stalePeers) > 0 {
		fmt.Printf("Cleaned up %d stale peers\n", len(stalePeers))
	}
}

// GetConnectedPeers returns all reachable peers
func (d *DHTService) GetConnectedPeers() []*PeerInfo {
	d.peerStoreMutex.RLock()
	defer d.peerStoreMutex.RUnlock()

	var peers []*PeerInfo
	for _, info := range d.peerStore {
		if info.IsReachable {
			peers = append(peers, info)
		}
	}
	return peers
}

// GetPeerCount returns the number of reachable peers
func (d *DHTService) GetPeerCount() int {
	return len(d.GetConnectedPeers())
}

// Errors returns a channel for receiving DHT errors
func (d *DHTService) Errors() <-chan error {
	return d.errChan
}

// Close shuts down the DHT service
func (d *DHTService) Close() error {
	d.cancel()
	d.wg.Wait()
	return d.dht.Close()
}

// getBootstrapPeers converts bootstrap addresses to peer.AddrInfo
func getBootstrapPeers() []peer.AddrInfo {
	var peers []peer.AddrInfo
	for _, addrStr := range DefaultBootstrapPeers {
		maddr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			continue
		}
		peers = append(peers, *peerInfo)
	}
	return peers
}
