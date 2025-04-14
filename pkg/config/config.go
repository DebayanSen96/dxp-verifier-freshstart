package config

// Config represents the configuration for the Dexponent verifier
type Config struct {
	// ListenAddresses are the addresses the verifier will listen on
	ListenAddresses []string `json:"listen_addresses"`

	// BootstrapPeers are the addresses of peers to connect to on startup
	BootstrapPeers []string `json:"bootstrap_peers"`

	// EnableMDNS enables multicast DNS for local peer discovery
	EnableMDNS bool `json:"enable_mdns"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ListenAddresses: []string{
			"/ip4/0.0.0.0/tcp/9000",
		},
		BootstrapPeers: []string{
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		},
		EnableMDNS: true,
	}
}
