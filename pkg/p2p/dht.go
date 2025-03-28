package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/multiformats/go-multiaddr"
)

// DefaultBootstrapPeers is a list of public DHT bootstrap nodes
var DefaultBootstrapPeers = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
}

// DHTService represents the DHT service for peer discovery
type DHTService struct {
	host host.Host
	dht  *dual.DHT
}

// NewDHT creates a new DHT service for peer discovery
func NewDHT(h host.Host) (*DHTService, error) {
	ctx := context.Background()

	// Create a dual DHT that operates both as a client and server
	dht, err := dual.New(ctx, h,
		dual.DHTOption(dht.Mode(dht.ModeServer)),
		dual.DHTOption(dht.BootstrapPeers(getBootstrapPeers()...)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	return &DHTService{
		host: h,
		dht:  dht,
	}, nil
}

// Bootstrap initiates connections to the bootstrap peers and bootstraps the DHT
func (d *DHTService) Bootstrap() error {
	ctx := context.Background()

	// Connect to bootstrap peers
	bootstrapPeers := getBootstrapPeers()
	fmt.Printf("Bootstrapping DHT with %d peers\n", len(bootstrapPeers))

	for _, peerInfo := range bootstrapPeers {
		if err := d.host.Connect(ctx, peerInfo); err != nil {
			fmt.Printf("Failed to connect to bootstrap peer %s: %v\n", peerInfo.ID, err)
		} else {
			fmt.Printf("Connected to bootstrap peer: %s\n", peerInfo.ID)
		}
	}

	// Bootstrap the DHT
	fmt.Println("Bootstrapping the DHT...")
	if err := d.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Start a routine to find peers
	go d.findPeersRoutine()

	return nil
}

// findPeersRoutine periodically searches for peers in the network
func (d *DHTService) findPeersRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("Searching for peers...")
			peers := d.host.Network().Peers()
			fmt.Printf("Connected to %d peers\n", len(peers))
			for _, p := range peers {
				fmt.Printf("  Peer: %s\n", p.String())
			}
		}
	}
}

// getBootstrapPeers converts the bootstrap peer addresses to multiaddrs
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
