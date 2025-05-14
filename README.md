# Dexponent Verifier Client

A robust verifier client for the Dexponent protocol built using Go Ethereum (Geth) with smart contract integration for consensus.

## Features

- **Smart Contract Integration**
  - Direct integration with Consensus smart contract
  - Event-driven architecture for round participation
  - Automatic submission of verification results
  - Efficient handling of concurrent verification rounds

- **Contract-Based Consensus**
  - Centralized consensus coordination via smart contract
  - Event subscription for round start notifications
  - Reliable submission of farm scores and benchmarks
  - Transparent verification process

- **Blockchain Integration**
  - Ethereum blockchain integration for verifier registration and rewards
  - Transaction status verification with proper error handling
  - Balance and allowance checking for DXP tokens

- **Usability**
  - Comprehensive command-line interface
  - Proper resource cleanup with wait groups and context cancellation
  - Detailed logging and status reporting

## Project Structure

```
dxp-verifier/
├── cmd/
│   └── verifier/      # Main executable
├── pkg/
│   ├── consensus/     # Contract-based consensus implementation
│   ├── dashboard/     # Web dashboard
│   └── eth/           # Ethereum client integration
├── POLKADOT_IMPLEMENTATION.md  # Implementation details based on Polkadot
└── dxp-verifier       # Executable binary
```

## File Descriptions

- `cmd/verifier/main.go`: Entry point for the application, handles command-line arguments and orchestrates components
- `pkg/config/config.go`: Configuration settings for the verifier
- `pkg/p2p/host.go`: Creates and configures the libp2p host with NAT traversal and multiple transports
- `pkg/p2p/dht.go`: Implements the dual Kademlia DHT for peer discovery across both LAN and WAN
- `pkg/p2p/mdns.go`: Implements mDNS for local peer discovery
- `pkg/p2p/protocol.go`: Implements the Dexponent protocol for message exchange and consensus
- `pkg/p2p/types.go`: Defines common types used across the P2P implementation
- `pkg/p2p/consensus.go`: Implements the consensus algorithm for verifier coordination
- `pkg/eth/client.go`: Ethereum client for blockchain interactions with proper transaction handling
- `pkg/eth/abi.go`: ABI definitions for interacting with the Dexponent smart contracts
- `POLKADOT_IMPLEMENTATION.md`: Documentation of the implementation details based on Polkadot's approach

## Getting Started

### Prerequisites

- Go 1.18 or higher
- Ethereum wallet with private key
- Access to an Ethereum RPC endpoint (Sepolia testnet)
- Minimum of 100 DXP tokens for verifier registration

### Building

```bash
go build -o dxp-verifier cmd/verifier/main.go
```

### Running

```bash
./dxp-verifier start
```

## Usage

### Environment Configuration

Create a `.env` file with the following variables:

```
# Ethereum RPC URL (Sepolia testnet)
NETWORK_RPC_URL=https://sepolia.infura.io/v3/YOUR_INFURA_KEY

# Contract address
DXP_CONTRACT_ADDRESS=0xYourContractAddress

# Wallet private key (without 0x prefix)
WALLET_PRIVATE_KEY=yourprivatekey

# Chain ID (11155111 for Sepolia)
CHAIN_ID=11155111

# Gas settings (optional)
GAS_LIMIT=3000000
GAS_PRICE_MULTIPLIER=1.1
```

### Available Commands

```
./dxp-verifier [command] [options]
```

Commands:
- `start`: Start the Dexponent verifier
  - `--block-polling-interval N`: Set block polling interval in seconds (default: 10)
  - `--detached`: Run in detached mode
- `register`: Register as a verifier with the DXP contract
  - `--amount N`: Amount of DXP tokens to stake (required)
  - `--farmid N`: Farm ID to register for (1-8, required)
- `status`: Check validator status and metrics
- `stop`: Stop a running validator
- `claim-rewards`: Claim accumulated rewards
- `send <key> <value>`: Send data to all connected Dexponent peers
- `withdraw`: Withdraw verifier stake
  - `--amount N`: Amount of stake to withdraw
