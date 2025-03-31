package eth

// DexponentProtocolABI is the ABI for the DexponentProtocol contract
const DexponentProtocolABI = `[
	{
		"inputs": [
			{
				"internalType": "bytes32",
				"name": "salt",
				"type": "bytes32"
			},
			{
				"internalType": "address",
				"name": "_dxpToken",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "fallbackRatio",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "_protocolFeeRate",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "_reserveRatio",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "constructor"
	},
	{
		"inputs": [],
		"name": "registerVerifier",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			}
		],
		"name": "registeredVerifiers",
		"outputs": [
			{
				"internalType": "bool",
				"name": "",
				"type": "bool"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "farmId",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "score",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "participants",
				"type": "address[]"
			}
		],
		"name": "submitVerification",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "verifier",
				"type": "address"
			}
		],
		"name": "getPendingRewards",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "claimRewards",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`
