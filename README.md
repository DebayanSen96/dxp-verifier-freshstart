# Dexponent Verifier Client

A basic verifier client for the Dexponent protocol built using Go Ethereum (Geth) and libp2p for peer-to-peer networking.

## Features

- Peer discovery using Kademlia DHT
- Local peer discovery using mDNS
- Automatic connection to bootstrap nodes
- Simple command-line interface

## Project Structure

```
dxp-verifier/
├── cmd/
│   └── verifier/      # Main executable
├── pkg/
│   ├── config/        # Configuration
│   └── p2p/           # P2P networking
└── dxp-verifier       # Executable script
```

## File Descriptions

- `cmd/verifier/main.go`: Entry point for the application, handles command-line arguments
- `pkg/config/config.go`: Configuration settings for the verifier
- `pkg/p2p/host.go`: Creates and configures the libp2p host
- `pkg/p2p/dht.go`: Implements the Kademlia DHT for peer discovery
- `pkg/p2p/mdns.go`: Implements mDNS for local peer discovery

## Getting Started

### Prerequisites

- Go 1.18 or higher

### Building

```bash
go build -o dxp-verifier cmd/verifier/main.go
```

### Running

```bash
./dxp-verifier start
```

Or use the provided script:

```bash
./dxp-verifier start
```

## Usage

When you start the verifier, it will:

1. Generate a new peer ID (or use an existing one if available)
2. Start listening for connections
3. Connect to bootstrap nodes
4. Start discovering peers using DHT and mDNS
5. Display connected peers

The verifier will continue running until you press Ctrl+C to stop it.
