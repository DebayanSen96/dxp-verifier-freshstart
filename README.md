# Dexponent Verifier Client

A basic verifier client for the Dexponent protocol built using Go Ethereum (Geth) and libp2p for peer-to-peer networking.

## Features

- Peer discovery using Kademlia DHT
- Local peer discovery using mDNS
- Automatic connection to bootstrap nodes
- Leader-based consensus mechanism with deterministic leader selection
- Ethereum blockchain integration for verifier registration and rewards
- Comprehensive command-line interface

## Project Structure

```
dxp-verifier/
├── cmd/
│   └── verifier/      # Main executable
├── pkg/
│   ├── config/        # Configuration
│   ├── eth/           # Ethereum client integration
│   └── p2p/           # P2P networking
└── dxp-verifier       # Executable script
```

## File Descriptions

- `cmd/verifier/main.go`: Entry point for the application, handles command-line arguments
- `pkg/config/config.go`: Configuration settings for the verifier
- `pkg/p2p/host.go`: Creates and configures the libp2p host
- `pkg/p2p/dht.go`: Implements the Kademlia DHT for peer discovery
- `pkg/p2p/mdns.go`: Implements mDNS for local peer discovery
- `pkg/p2p/protocol.go`: Implements the Dexponent protocol for consensus
- `pkg/eth/client.go`: Ethereum client for blockchain interactions

## Getting Started

### Prerequisites

- Go 1.18 or higher
- Ethereum wallet with private key
- Access to an Ethereum RPC endpoint (Sepolia testnet)

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

### Environment Configuration

Create a `.env` file with the following variables:

```
# Sepolia testnet RPC URL
BASE_RPC_URL=https://sepolia.infura.io/v3/YOUR_API_KEY

# DXP contract address on Sepolia testnet
DXP_CONTRACT_ADDRESS=0x8437ab3cCb485D2a3793F97f58c6e3F926039684

# Validator wallet private key (without 0x prefix)
WALLET_PRIVATE_KEY=YOUR_PRIVATE_KEY_HERE

# Gas price multiplier (default: 1.0)
GAS_PRICE_MULTIPLIER=1.1

# Gas limit for transactions (default: 3000000)
GAS_LIMIT=3000000

# Chain ID for Sepolia testnet (default: 11155111)
CHAIN_ID=11155111
```

### Available Commands

```bash
# Start the verifier
./dxp-verifier start [--block-polling-interval N] [--detached]

# Register as a verifier with the DXP contract
./dxp-verifier register

# Check validator status
./dxp-verifier status

# Stop a running validator
./dxp-verifier stop

# Check pending rewards
./dxp-verifier rewards

# Claim accumulated rewards
./dxp-verifier claim

# Send data to all connected Dexponent peers
./dxp-verifier send <key> <value>
```

### Starting the Verifier

When you start the verifier, it will:

1. Generate a new peer ID (or use an existing one if available)
2. Start listening for connections
3. Connect to bootstrap nodes
4. Start discovering peers using DHT and mDNS
5. Initialize the Ethereum client for blockchain interactions
6. Begin the consensus process for farm performance validation
7. Display connected peers and consensus status

The verifier will continue running until you press Ctrl+C to stop it or use the `stop` command.
