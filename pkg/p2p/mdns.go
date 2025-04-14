package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// MDNSService represents the mDNS discovery service
type MDNSService struct {
	host host.Host
	mdns mdns.Service
}

// mdnsNotifee gets notified when we find a new peer via mDNS discovery
type mdnsNotifee struct {
	host host.Host
}

// HandlePeerFound connects to peers discovered via mDNS
func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Skip if it's our own peer ID
	if pi.ID == n.host.ID() {
		return
	}

	fmt.Printf("mDNS: discovered new peer: %s\n", pi.ID.String())

	// Connect to the discovered peer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := n.host.Connect(ctx, pi); err != nil {
		fmt.Printf("mDNS: failed to connect to peer %s: %v\n", pi.ID.String(), err)
	} else {
		fmt.Printf("mDNS: successfully connected to peer %s\n", pi.ID.String())
	}
}

// NewMDNS creates a new mDNS discovery service
func NewMDNS(h host.Host) (*MDNSService, error) {
	// Create a new notifee
	notifee := &mdnsNotifee{host: h}

	// Create a new mDNS service
	service := mdns.NewMdnsService(h, "dexponent-verifier", notifee)
	if err := service.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mDNS service: %w", err)
	}

	return &MDNSService{
		host: h,
		mdns: service,
	}, nil
}
