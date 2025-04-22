// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "@openzeppelin/contracts/utils/cryptography/MessageHashUtils.sol";

/**
 * @title Consensus
 * @notice Contract for verifier management, farm scoring, and reward distribution
 */
contract Consensus is ReentrancyGuard, Ownable {
    uint256 public constant MIN_VERIFIER_STAKE = 100;
    IERC20 public dxpToken;
    
    mapping(address => uint256) public verifierStake;
    mapping(address => bool) public registeredVerifiers;
    
    mapping(uint256 => mapping(address => bool)) public farmRegisteredVerifiers;
    mapping(uint256 => address[]) public registeredVerifiersForFarm;
    mapping(uint256 => mapping(address => bool)) public farmActiveVerifiers;
    mapping(uint256 => address[]) public activeVerifiersForFarm;
    
    mapping(uint256 => uint256) public farmBenchmarks;
    mapping(uint256 => uint256) public farmScores;
    mapping(uint256 => uint256) public lastScoreUpdate;
    
    mapping(uint256 => address) public farmLeader;
    mapping(uint256 => uint256) public farmConsensusRound;
    

    event FarmLeaderRegistered(uint256 indexed farmId, address indexed leader, uint256 indexed roundNumber);
    event FarmConsensusCompleted(uint256 indexed farmId, uint256 indexed roundNumber, address indexed leader);


    struct VerifierMetrics {
        uint256 verificationsPerformed;
        uint256 lastActiveTimestamp;
        uint256 totalUptime;
        uint256[] assignedFarms;
    }
    mapping(address => VerifierMetrics) public verifierMetrics;
    

    
    event VerifierRegistered(address indexed verifier, uint256 stake, uint256 indexed farmId);
    event VerifierActivated(address indexed verifier, uint256 indexed farmId);
    event VerifierDeactivated(address indexed verifier, uint256 indexed farmId);
    event VerifierStakeIncreased(address indexed verifier, uint256 additionalStake, uint256 newTotalStake);
    event VerifierStakeWithdrawn(address indexed verifier, uint256 amount, uint256 remainingStake);
    event VerifierDeregistered(address indexed verifier, uint256 stake, string reason);
    event FarmScoreUpdated(uint256 indexed farmId, uint256 newScore, address indexed updatedBy);
    event FarmBenchmarkUpdated(uint256 indexed farmId, uint256 newBenchmark, address indexed updatedBy);

    event VerifierMetricsUpdated(address indexed verifier, uint256 verificationsPerformed, uint256 uptime);

    /**
     * @notice Constructor to initialize the contract with the deployer as the owner and set the DXP token address
     * @param _dxpTokenAddress The address of the DXP token contract
     */
    constructor(address _dxpTokenAddress) Ownable(msg.sender) {
        require(_dxpTokenAddress != address(0), "DXP token address cannot be zero");
        dxpToken = IERC20(_dxpTokenAddress);
        
        for (uint256 i = 1; i <= 8; i++) {
            farmBenchmarks[i] = 1000; 
            farmScores[i] = 0.5 * 1e18; 
            emit FarmBenchmarkUpdated(i, 1000, msg.sender);
        }
    }

    /**
     * @notice Register as the leader for a farm consensus round
     * @param farmId The ID of the farm for which to register as leader
     */
    function registerFarmLeader(uint256 farmId) 
        external 
        onlyRegisteredVerifier 
        onlyFarmRegisteredVerifier(farmId)
        validFarmId(farmId) 
    {
        if (farmLeader[farmId] != address(0)) {
 
            if (farmLeader[farmId] == msg.sender) {
                farmConsensusRound[farmId]++;
                emit FarmLeaderRegistered(farmId, msg.sender, farmConsensusRound[farmId]);
                return;
            }

            revert("Farm already has a different leader");
        }
        
        // No leader yet, register as the leader
        farmLeader[farmId] = msg.sender;
        
        // Increment the round counter
        farmConsensusRound[farmId]++;
        
        VerifierMetrics storage metrics = verifierMetrics[msg.sender];
        metrics.verificationsPerformed += 1;
        metrics.lastActiveTimestamp = block.timestamp;
        
        emit FarmLeaderRegistered(farmId, msg.sender, farmConsensusRound[farmId]);
    }

 
    modifier onlyRegisteredVerifier() {
        require(registeredVerifiers[msg.sender], "Not a registered verifier");
        _;
    }
    
    modifier onlyFarmRegisteredVerifier(uint256 farmId) {
        require(farmRegisteredVerifiers[farmId][msg.sender], "Not registered for this farm");
        _;
    }
    
    modifier onlyFarmActiveVerifier(uint256 farmId) {
        require(farmActiveVerifiers[farmId][msg.sender], "Not an active verifier for this farm");
        _;
    }
    
    modifier onlyFarmLeader(uint256 farmId) {
        require(farmLeader[farmId] == msg.sender, "Not the current leader for this farm");
        _;
    }
    
    modifier validFarmId(uint256 farmId) {
        require(farmId >= 1 && farmId <= 8, "Invalid farm ID (must be 1-8)");
        _;
    }

    /**
     * @notice Register as a verifier with a specified stake amount and farm ID
     * @param verifierAddress The address to register as a verifier
     * @param stakeAmount The amount of DXP tokens to stake (must be >= MIN_VERIFIER_STAKE)
     * @param farmId The ID of the farm to register for (1-8)
     */
    function registerVerifier(address verifierAddress, uint256 stakeAmount, uint256 farmId) 
        external 
        nonReentrant 
        validFarmId(farmId) 
    {
        require(!registeredVerifiers[verifierAddress], "Already registered as verifier");
        require(stakeAmount >= MIN_VERIFIER_STAKE, "Insufficient stake amount");
        require(dxpToken.allowance(msg.sender, address(this)) >= stakeAmount, "Insufficient DXP token allowance");
        
        bool success = dxpToken.transferFrom(msg.sender, address(this), stakeAmount);
        require(success, "DXP token transfer failed");
        
        registeredVerifiers[verifierAddress] = true;
        verifierStake[verifierAddress] = stakeAmount;
        
        farmRegisteredVerifiers[farmId][verifierAddress] = true;
        registeredVerifiersForFarm[farmId].push(verifierAddress);
        
        farmActiveVerifiers[farmId][verifierAddress] = true;
        activeVerifiersForFarm[farmId].push(verifierAddress);
        
        VerifierMetrics storage metrics = verifierMetrics[verifierAddress];
        metrics.lastActiveTimestamp = block.timestamp;
        metrics.assignedFarms.push(farmId);
        
        emit VerifierRegistered(verifierAddress, stakeAmount, farmId);
        emit VerifierActivated(verifierAddress, farmId);
    }

    /**
     * @notice Activate a verifier for a specific farm (owner only)
     * @param verifier The address of the verifier to activate
     * @param farmId The ID of the farm
     */
    function activateVerifier(address verifier, uint256 farmId) 
        external 
        onlyOwner 
        validFarmId(farmId) 
    {
        require(farmRegisteredVerifiers[farmId][verifier], "Not registered for this farm");
        require(!farmActiveVerifiers[farmId][verifier], "Already active for this farm");
        require(_isActiveVerifier(verifier), "Not an active verifier with sufficient stake");
        
        farmActiveVerifiers[farmId][verifier] = true;
        activeVerifiersForFarm[farmId].push(verifier);
        
        emit VerifierActivated(verifier, farmId);
    }
    
    /**
     * @notice Deactivate a verifier for a specific farm
     * @param verifier The address of the verifier to deactivate
     * @param farmId The ID of the farm
     */
    function deactivateVerifier(address verifier, uint256 farmId) 
        external 
        onlyOwner 
        validFarmId(farmId) 
    {
        require(farmActiveVerifiers[farmId][verifier], "Not active for this farm");
        
        farmActiveVerifiers[farmId][verifier] = false;
        
     
        for (uint i = 0; i < activeVerifiersForFarm[farmId].length; i++) {
            if (activeVerifiersForFarm[farmId][i] == verifier) {
                activeVerifiersForFarm[farmId][i] = activeVerifiersForFarm[farmId][activeVerifiersForFarm[farmId].length - 1];
                activeVerifiersForFarm[farmId].pop();
                break;
            }
        }
        
        emit VerifierDeactivated(verifier, farmId);
    }

    /**
     * @notice Increase stake for a verifier
     * @param verifierAddress The address of the verifier
     * @param additionalStake The additional amount of DXP tokens to stake
     */
    function increaseVerifierStake(address verifierAddress, uint256 additionalStake) external nonReentrant onlyRegisteredVerifier {
        require(registeredVerifiers[verifierAddress], "Not registered as verifier");
        require(additionalStake > 0, "No stake provided");
        require(dxpToken.allowance(msg.sender, address(this)) >= additionalStake, "Insufficient DXP token allowance");
        bool success = dxpToken.transferFrom(msg.sender, address(this), additionalStake);
        require(success, "DXP token transfer failed");
        
        uint256 newTotalStake = verifierStake[verifierAddress] + additionalStake;
        verifierStake[verifierAddress] = newTotalStake;
        
        emit VerifierStakeIncreased(verifierAddress, additionalStake, newTotalStake);
    }

    /**
     * @notice Withdraw stake for a verifier
     * @param verifierAddress The address of the verifier
     * @param amount The amount of DXP tokens to withdraw
     */
    function withdrawVerifierStake(address verifierAddress, uint256 amount) external nonReentrant onlyRegisteredVerifier {
        require(msg.sender == verifierAddress, "Only the verifier can withdraw their stake");
        require(registeredVerifiers[verifierAddress], "Not registered as verifier");
        require(amount > 0 && amount <= verifierStake[verifierAddress], "Invalid withdrawal amount");
        
        uint256 remainingStake = verifierStake[verifierAddress] - amount;
        

        if (remainingStake > 0) {
            require(remainingStake >= MIN_VERIFIER_STAKE, "Remaining stake below minimum");
            verifierStake[verifierAddress] = remainingStake;
        } else {
         
            _deregisterVerifier(verifierAddress, "Full stake withdrawal");
        }
        
       
        bool success = dxpToken.transfer(verifierAddress, amount);
        require(success, "DXP token transfer failed");
        
        emit VerifierStakeWithdrawn(verifierAddress, amount, remainingStake);
    }

    /**
     * @notice Submit a farm score with cryptographic verification using ECDSA (only by the current farm leader)
     * @param farmId The ID of the farm
     * @param score The score value
     * @param signature ECDSA signature of the score data
     */
    function submitFarmScore(
        uint256 farmId, 
        uint256 score,
        bytes memory signature
    ) 
        external 
        onlyRegisteredVerifier
        onlyFarmRegisteredVerifier(farmId) 
        onlyFarmLeader(farmId)
        validFarmId(farmId) 
    {
        require(score <= 1e18, "Score cannot exceed 1.0");

        bytes32 messageHash = keccak256(abi.encodePacked(farmId, score, msg.sender));
        
        bytes32 ethSignedMessageHash = MessageHashUtils.toEthSignedMessageHash(messageHash);
        
        address signer = ECDSA.recover(ethSignedMessageHash, signature);
        
        require(signer == msg.sender, "Invalid signature");
        
        farmScores[farmId] = score;
        lastScoreUpdate[farmId] = block.timestamp;

        VerifierMetrics storage metrics = verifierMetrics[msg.sender];
        metrics.verificationsPerformed++;

        uint256 timeSinceLastActive = block.timestamp - metrics.lastActiveTimestamp;
        metrics.totalUptime += timeSinceLastActive;
        metrics.lastActiveTimestamp = block.timestamp;

        uint256 currentRound = farmConsensusRound[farmId];

        emit FarmScoreUpdated(farmId, score, msg.sender);
        emit FarmConsensusCompleted(farmId, currentRound, msg.sender);
        emit VerifierMetricsUpdated(msg.sender, metrics.verificationsPerformed, metrics.totalUptime);
    }

    /**
     * @notice Set farm benchmark with cryptographic verification using ECDSA (only by the current farm leader)
     * @param farmId The ID of the farm
     * @param benchmark The benchmark value (in basis points, 10000 = 100%)
     * @param signature ECDSA signature of the benchmark data
     */
    function setFarmBenchmarkSecure(
        uint256 farmId, 
        uint256 benchmark,
        bytes memory signature
    ) 
        external 
        onlyRegisteredVerifier
        onlyFarmRegisteredVerifier(farmId) 
        onlyFarmLeader(farmId)
        validFarmId(farmId) 
    {
        require(benchmark <= 10000, "Benchmark cannot exceed 100%");
        
       
        bytes32 messageHash = keccak256(abi.encodePacked(farmId, benchmark, msg.sender));
        
       
        bytes32 ethSignedMessageHash = MessageHashUtils.toEthSignedMessageHash(messageHash);
        
        address signer = ECDSA.recover(ethSignedMessageHash, signature);
        
        require(signer == msg.sender, "Invalid signature");
        
        farmBenchmarks[farmId] = benchmark;
        
        VerifierMetrics storage metrics = verifierMetrics[msg.sender];
        metrics.verificationsPerformed++;
        
        uint256 timeSinceLastActive = block.timestamp - metrics.lastActiveTimestamp;
        metrics.totalUptime += timeSinceLastActive;
        metrics.lastActiveTimestamp = block.timestamp;
        
        uint256 currentRound = farmConsensusRound[farmId];
        
        emit FarmBenchmarkUpdated(farmId, benchmark, msg.sender);
        emit FarmConsensusCompleted(farmId, currentRound, msg.sender);
        emit VerifierMetricsUpdated(msg.sender, metrics.verificationsPerformed, metrics.totalUptime);
    }

    /**
     * @notice Internal function to deregister a verifier
     * @param verifierAddress The address of the verifier to deregister
     * @param reason The reason for deregistration
     */
    function _deregisterVerifier(address verifierAddress, string memory reason) internal {
        uint256 currentStake = verifierStake[verifierAddress];
        registeredVerifiers[verifierAddress] = false;
        delete verifierStake[verifierAddress];
        
       
        VerifierMetrics storage metrics = verifierMetrics[verifierAddress];
        for (uint i = 0; i < metrics.assignedFarms.length; i++) {
            uint256 farmId = metrics.assignedFarms[i];
            farmRegisteredVerifiers[farmId][verifierAddress] = false;
            farmActiveVerifiers[farmId][verifierAddress] = false;
        }
        
        emit VerifierDeregistered(verifierAddress, currentStake, reason);
    }

    /**
     * @notice Check if an address is a registered verifier with sufficient stake
     * @param verifier The address to check
     * @return True if the address is a registered verifier with sufficient stake
     */
    function _isActiveVerifier(address verifier) internal view returns (bool) {
        return registeredVerifiers[verifier] && verifierStake[verifier] >= MIN_VERIFIER_STAKE;
    }

    /**
     * @notice Check if an address is a registered verifier with sufficient stake
     * @param verifier The address to check
     * @return True if the address is a registered verifier with sufficient stake
     */
    function isVerifier(address verifier) external view returns (bool) {
        return _isActiveVerifier(verifier);
    }

    /**
     * @notice Check if a verifier is registered for a specific farm
     * @param farmId The ID of the farm
     * @param verifier The address of the verifier
     * @return True if the verifier is registered for the farm
     */
    function isVerifierRegisteredForFarm(uint256 farmId, address verifier) external view returns (bool) {
        return farmRegisteredVerifiers[farmId][verifier] && _isActiveVerifier(verifier);
    }
    
    /**
     * @notice Check if a verifier is active for a specific farm
     * @param farmId The ID of the farm
     * @param verifier The address of the verifier
     * @return True if the verifier is active for the farm
     */
    function isVerifierActiveForFarm(uint256 farmId, address verifier) external view returns (bool) {
        return farmActiveVerifiers[farmId][verifier] && _isActiveVerifier(verifier);
    }

    /**
     * @notice Get the stake amount of a verifier
     * @param verifier The address of the verifier
     * @return The stake amount in DXP tokens
     */
    function getVerifierStake(address verifier) external view returns (uint256) {
        return verifierStake[verifier];
    }

    /**
     * @notice Get all registered verifiers for a farm
     * @param farmId The ID of the farm
     * @return Array of verifier addresses
     */
    function getRegisteredVerifiersForFarm(uint256 farmId) external view validFarmId(farmId) returns (address[] memory) {
        return registeredVerifiersForFarm[farmId];
    }
    
    /**
     * @notice Get all active verifiers for a farm
     * @param farmId The ID of the farm
     * @return Array of verifier addresses
     */
    function getActiveVerifiersForFarm(uint256 farmId) external view validFarmId(farmId) returns (address[] memory) {
        return activeVerifiersForFarm[farmId];
    }
    
    /**
     * @notice Get verifier metrics
     * @param verifier The address of the verifier
     * @return verificationsPerformed Number of verifications performed
     * @return lastActiveTimestamp Last active timestamp
     * @return totalUptime Total uptime in seconds
     */
    function getVerifierMetrics(address verifier) external view returns (
        uint256 verificationsPerformed,
        uint256 lastActiveTimestamp,
        uint256 totalUptime
    ) {
        VerifierMetrics storage metrics = verifierMetrics[verifier];
        return (
            metrics.verificationsPerformed,
            metrics.lastActiveTimestamp,
            metrics.totalUptime
        );
    }
    
    /**
     * @notice Get current farm data
     * @param farmId The ID of the farm
     * @return score Current farm score
     * @return benchmark Current farm benchmark
     * @return lastUpdate Last update timestamp
     */
    function getFarmData(uint256 farmId) external view validFarmId(farmId) returns (
        uint256 score,
        uint256 benchmark,
        uint256 lastUpdate
    ) {
        return (
            farmScores[farmId],
            farmBenchmarks[farmId],
            lastScoreUpdate[farmId]
        );
    }
    

    
    /**
     * @notice Check and update verifier status if stake is below minimum
     * @param verifierAddress The address of the verifier to check
     * @return True if the verifier is still active after the check
     */
    function checkAndUpdateVerifierStatus(address verifierAddress) external onlyOwner returns (bool) {
        if (registeredVerifiers[verifierAddress] && verifierStake[verifierAddress] < MIN_VERIFIER_STAKE) {
            _deregisterVerifier(verifierAddress, "Stake below minimum requirement");
            return false;
        }
        return _isActiveVerifier(verifierAddress);
    }
    
    /**
     * @notice Get the current consensus round number for a farm
     * @param farmId The ID of the farm
     * @return The current consensus round number
     */
    function getCurrentFarmRound(uint256 farmId) external view validFarmId(farmId) returns (uint256) {
        return farmConsensusRound[farmId];
    }
    
    /**
     * @notice Check if a farm has an active leader
     * @param farmId The ID of the farm
     * @return True if the farm has an active leader, false otherwise
     */
    function hasFarmLeader(uint256 farmId) external view validFarmId(farmId) returns (bool) {
        return farmLeader[farmId] != address(0);
    }
    
    /**
     * @notice Get the current leader for a farm
     * @param farmId The ID of the farm
     * @return The address of the current leader, or zero address if none
     */
    function getFarmLeader(uint256 farmId) external view validFarmId(farmId) returns (address) {
        return farmLeader[farmId];
    }
}