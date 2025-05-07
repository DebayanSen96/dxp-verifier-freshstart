// Global variables
let pendingRewardsEl;
let claimRewardsBtn;
let confirmWithdrawBtn;
let withdrawAmount;
let actionStatus;
let refreshBtn;
let toggleNodeBtn;
let nodeStatusIndicator;
let terminalContainer;
let terminalOutput;
let nodeOutputPollInterval;

// ASCII art for DEXPONENT
const dexponentAscii = [
"██████╗ ███████╗██╗  ██╗██████╗  ██████╗ ███╗   ██╗███████╗███╗   ██╗████████╗",
"██╔══██╗██╔════╝╚██╗██╔╝██╔══██╗██╔═══██╗████╗  ██║██╔════╝████╗  ██║╚══██╔══╝",
"██║  ██║█████╗   ╚███╔╝ ██████╔╝██║   ██║██╔██╗ ██║█████╗  ██╔██╗ ██║   ██║   ",
"██║  ██║██╔══╝   ██╔██╗ ██╔═══╝ ██║   ██║██║╚██╗██║██╔══╝  ██║╚██╗██║   ██║   ",
"██████╔╝███████╗██╔╝ ██╗██║     ╚██████╔╝██║ ╚████║███████╗██║ ╚████║   ██║   ",
"╚═════╝ ╚══════╝╚═╝  ╚═╝╚═╝      ╚═════╝ ╚═╝  ╚═══╝╚══════╝╚═╝  ╚═══╝   ╚═╝   "
];

// Hacker-style phrases to display during loading
const hackerPhrases = [
    "Initializing secure connection...",
    "Verifying blockchain integrity...",
    "Establishing P2P network...",
    "Synchronizing with DXP protocol...",
    "Loading dashboard..."
];

// Function to animate ASCII art
function animateAscii() {
    const asciiEl = document.getElementById('ascii-animation');
    if (!asciiEl) return;
    
    // Clear any existing content
    asciiEl.innerHTML = '';
    
    // First animation: Typing effect for the ASCII art
    let lineIndex = 0;
    let charIndex = 0;
    
    function typeAsciiArt() {
        if (lineIndex < dexponentAscii.length) {
            if (charIndex === 0) {
                // Create a new line
                const line = document.createElement('div');
                line.className = 'ascii-line';
                asciiEl.appendChild(line);
            }
            
            const currentLine = asciiEl.querySelectorAll('.ascii-line')[lineIndex];
            
            if (charIndex < dexponentAscii[lineIndex].length) {
                // Add next character
                currentLine.textContent += dexponentAscii[lineIndex][charIndex];
                charIndex++;
                setTimeout(typeAsciiArt, 5); // Type each character quickly
            } else {
                // Move to next line
                lineIndex++;
                charIndex = 0;
                setTimeout(typeAsciiArt, 50); // Small pause between lines
            }
        } else {
            // Start the glitch effect after typing is complete
            setTimeout(glitchEffect, 500);
        }
    }
    
    // Second animation: Glitch effect
    function glitchEffect() {
        const lines = asciiEl.querySelectorAll('.ascii-line');
        let glitchCount = 0;
        const maxGlitches = 10;
        
        const glitchInterval = setInterval(() => {
            if (glitchCount >= maxGlitches) {
                clearInterval(glitchInterval);
                
                // Reset to original ASCII art
                lines.forEach((line, index) => {
                    line.textContent = dexponentAscii[index];
                });
                
                // Start matrix rain effect
                setTimeout(() => {
                    // Pulse effect on the ASCII art
                    asciiEl.classList.add('pulse');
                    
                    // Start hacker phrases
                    animateHackerPhrases();
                }, 500);
                
                return;
            }
            
            // Apply glitch to random line
            const randomLineIndex = Math.floor(Math.random() * lines.length);
            const originalText = dexponentAscii[randomLineIndex];
            const line = lines[randomLineIndex];
            
            // Create glitched text by replacing random characters
            let glitchedText = '';
            for (let i = 0; i < originalText.length; i++) {
                if (Math.random() < 0.1) { // 10% chance to glitch each character
                    const glitchChars = '!@#$%^&*()_+-=[]{}|;:,.<>?/\\';
                    glitchedText += glitchChars[Math.floor(Math.random() * glitchChars.length)];
                } else {
                    glitchedText += originalText[i];
                }
            }
            
            line.textContent = glitchedText;
            
            // Reset line after short delay
            setTimeout(() => {
                line.textContent = originalText;
            }, 100);
            
            glitchCount++;
        }, 200);
    }
    
    // Start the typing animation
    typeAsciiArt();
}

// Function to animate hacker phrases
function animateHackerPhrases() {
    const loadingText = document.querySelector('.loading-text');
    if (!loadingText) return;
    
    let phraseIndex = 0;
    
    function showNextPhrase() {
        if (phraseIndex < hackerPhrases.length) {
            // Fade out current text
            loadingText.style.opacity = '0';
            
            // Change text and fade in after a short delay
            setTimeout(() => {
                loadingText.textContent = hackerPhrases[phraseIndex];
                loadingText.style.opacity = '1';
                phraseIndex++;
                
                // Show next phrase after a delay
                setTimeout(showNextPhrase, 400);
            }, 200);
        } else {
            // All phrases shown, hide loading screen after a delay
            setTimeout(hideLoadingScreen, 500);
        }
    }
    
    // Start showing phrases
    showNextPhrase();
}

// Function to hide loading screen
function hideLoadingScreen() {
    const loadingOverlay = document.getElementById('loading-overlay');
    if (loadingOverlay) {
        loadingOverlay.classList.add('hidden');
        
        // Remove from DOM after transition completes
        setTimeout(() => {
            loadingOverlay.style.display = 'none';
        }, 500);
    }
}

document.addEventListener('DOMContentLoaded', function() {
    // Start ASCII animation
    animateAscii();
    
    // Elements
    pendingRewardsEl = document.getElementById('pending-rewards');
    claimRewardsBtn = document.getElementById('claim-rewards-btn');
    confirmWithdrawBtn = document.getElementById('confirm-withdraw-btn');
    withdrawAmount = document.getElementById('withdraw-amount');
    actionStatus = document.getElementById('action-status');
    refreshBtn = document.getElementById('refresh-btn');
    toggleNodeBtn = document.getElementById('toggle-node-btn');
    nodeStatusIndicator = document.getElementById('node-status-indicator');
    terminalContainer = document.getElementById('terminal-container');
    terminalOutput = document.getElementById('terminal-output');

    // Debug element references
    console.log('Elements loaded:', {
        pendingRewardsEl: pendingRewardsEl,
        claimRewardsBtn: claimRewardsBtn,
        confirmWithdrawBtn: confirmWithdrawBtn,
        withdrawAmount: withdrawAmount,
        actionStatus: actionStatus,
        refreshBtn: refreshBtn,
        toggleNodeBtn: toggleNodeBtn,
        nodeStatusIndicator: nodeStatusIndicator,
        terminalContainer: terminalContainer,
        terminalOutput: terminalOutput
    });

    // Fetch initial status
    fetchStatus();

    // Set up event listeners
    if (claimRewardsBtn) {
        claimRewardsBtn.addEventListener('click', claimRewards);
        console.log('Claim rewards button listener attached');
    }
    
    if (toggleNodeBtn) {
        toggleNodeBtn.addEventListener('click', toggleNode);
        console.log('Toggle node button listener attached');
    }

    // We're using onclick in HTML, so don't add another event listener here
    // to avoid duplicate calls
    
    if (refreshBtn) {
        refreshBtn.addEventListener('click', function(e) {
            e.preventDefault();
            fetchStatus();
            showStatus('Refreshed data', 'success');
        });
    }

    // Auto-refresh every 30 seconds
    setInterval(fetchStatus, 30000);

    // Initialize verification chart
    initVerificationChart();
});

// Fetch status from API
function fetchStatus() {
    fetch('/api/status')
        .then(response => response.json())
        .then(data => {
            if (pendingRewardsEl) {
                pendingRewardsEl.textContent = data.pendingRewards || '0 DXP';
            }
        })
        .catch(error => {
            console.error('Error fetching status:', error);
        });
}

function claimRewards() {
    fetch('/api/claim-rewards', {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showStatus(`Error: ${data.error}`, 'error');
        } else {
            showStatus(`Success: ${data.status}. Transaction: ${data.txHash}`, 'success');
            // Refresh status after a short delay
            setTimeout(fetchStatus, 2000);
        }
    })
    .catch(error => {
        showStatus(`Error: ${error.message}`, 'error');
    });
}

// Make the function globally accessible
window.withdrawStake = function() {
    console.log('[withdrawStake] Called.'); // Log 1

    // Validate wallet address
    const walletAddressElement = document.getElementById('wallet-address-display');
    console.log('[withdrawStake] walletAddressElement (DOM object):', walletAddressElement);

    if (!walletAddressElement) {
        showStatus('Error: Wallet address display element not found in DOM. Cannot proceed.', 'error');
        return;
    }

    const walletAddress = walletAddressElement.textContent.trim();
    console.log('[withdrawStake] walletAddress (text content):', walletAddress);

    if (!walletAddress || walletAddress === 'N/A' || walletAddress === '') {
        showStatus('Error: Wallet address not available or N/A. Please connect wallet. Value: "' + walletAddress + '"', 'error');
        return;
    }

    // Validate current stake
    const currentStakeElement = document.getElementById('current-stake-value');
    console.log('[withdrawStake] currentStakeElement (DOM object with ID current-stake-value):', currentStakeElement);

    if (!currentStakeElement) {
        showStatus('Error: Current stake display element (ID: current-stake-value) not found in DOM.', 'error');
        return;
    }
    
    const currentStakeText = currentStakeElement.textContent.trim();
    console.log('[withdrawStake] currentStakeText (from element content):', currentStakeText);
    
    let currentStake;
    try {
        currentStake = parseFloat(currentStakeText);
        if (isNaN(currentStake)) {
            // Try to see if it's a BigNumber string if parseFloat fails (e.g. has ' ETH' or similar)
            const cleanedStakeText = currentStakeText.replace(/[^0-9.]/g, '');
            currentStake = parseFloat(cleanedStakeText);
        }
    } catch (e) {
        console.error('[withdrawStake] Error parsing currentStakeText:', e);
        showStatus('Error: Could not parse current stake value: ' + currentStakeText, 'error');
        return;
    }
    console.log('[withdrawStake] currentStake (parsed float):', currentStake);

    if (isNaN(currentStake) || currentStake <= 0) { 
        showStatus('Error: Invalid or zero current stake (parsed as ' + currentStake + ' from text "' + currentStakeText + '"). Cannot initiate withdrawal.', 'error');
        return;
    }

    // If all checks pass, focus on the withdraw amount input
    console.log('[withdrawStake] All prerequisite checks passed. Current stake: ' + currentStake);
    document.getElementById('withdraw-amount').focus();
    showStatus('Enter amount to withdraw and click Confirm Withdrawal', 'info');
};

// Function to handle the withdrawal confirmation directly from the form
window.confirmWithdrawal = function() {
    console.log('[confirmWithdrawal] Button clicked, function triggered.');
    
    // Get the withdrawal amount from the input field
    const withdrawAmountInput = document.getElementById('withdraw-amount');
    if (!withdrawAmountInput) {
        showStatus('Error: Withdrawal amount input field not found.', 'error');
        return;
    }
    
    const amountStr = withdrawAmountInput.value.trim();
    console.log('[confirmWithdrawal] Amount entered:', amountStr);
    
    if (!amountStr) {
        showStatus('Please enter an amount to withdraw.', 'error');
        return;
    }
    
    // Parse and validate the amount
    const amount = parseFloat(amountStr);
    if (isNaN(amount) || amount <= 0) {
        showStatus('Please enter a valid positive number.', 'error');
        return;
    }
    
    // Get the current stake from the page
    const currentStakeElement = document.getElementById('current-stake-value');
    const currentStake = parseFloat(currentStakeElement.textContent.trim());
    console.log('[confirmWithdrawal] Current stake:', currentStake);
    
    if (amount > currentStake) {
        showStatus(`Cannot withdraw more than your current stake of ${currentStake} DXP.`, 'error');
        return;
    }
    
    // Check minimum stake requirements
    const minimumStake = 100; // DXP
    const remainingStake = currentStake - amount;
    
    // If remaining stake would be below minimum but greater than zero, confirm full withdrawal
    if (remainingStake > 0 && remainingStake < minimumStake) {
        const confirmFullWithdrawal = confirm(
            `Withdrawing ${amount} DXP would leave your stake at ${remainingStake.toFixed(6)} DXP, which is below the minimum requirement of ${minimumStake} DXP.\n\n` +
            `You can either:\n- Withdraw up to ${(currentStake - minimumStake).toFixed(6)} DXP to maintain the minimum stake\n- Withdraw your full stake of ${currentStake} DXP\n\n` +
            `Would you like to proceed with a full withdrawal instead?`
        );
        
        if (confirmFullWithdrawal) {
            // User chose to withdraw everything
            console.log('[confirmWithdrawal] Switching to full withdrawal');
            withdrawAmountInput.value = currentStake.toString();
        } else {
            // User canceled
            return;
        }
    }
    
    // Get the final amount (in case it was updated for full withdrawal)
    const finalAmount = parseFloat(withdrawAmountInput.value.trim());
    console.log('[confirmWithdrawal] Final withdrawal amount:', finalAmount);
    
    // Determine if this is a full withdrawal
    const isFullWithdrawal = Math.abs(finalAmount - currentStake) < 0.000001;
    console.log('[confirmWithdrawal] Is full withdrawal:', isFullWithdrawal);
    
    // Disable the confirm button to prevent multiple submissions
    const confirmButton = document.getElementById('confirm-withdraw-btn');
    if (confirmButton) {
        confirmButton.disabled = true;
        confirmButton.textContent = 'Processing...';
    }
    
    // Show processing status
    showStatus('Processing withdrawal request...', 'info');
    
    // Make the API call
    console.log('[confirmWithdrawal] Sending API request to /api/withdraw with data:', { amount: finalAmount.toString() });
    
    // Get the assigned farm ID if available
    let farmId = '';
    const assignedFarmsElement = document.querySelector('.metric:nth-child(3) .value');
    if (assignedFarmsElement) {
        const farmsText = assignedFarmsElement.textContent.trim();
        if (farmsText && farmsText !== 'None') {
            // Extract the first farm ID if multiple are listed
            const farmMatch = farmsText.match(/\d+/);
            if (farmMatch) {
                farmId = farmMatch[0];
                console.log('[confirmWithdrawal] Using farm ID:', farmId);
            }
        }
    }
    
    // Create URL-encoded form data
    const formData = new URLSearchParams();
    formData.append('amount', finalAmount.toString());
    if (farmId) {
        formData.append('farmId', farmId);
    }
    
    fetch('/api/withdraw', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
        },
        body: formData.toString()
    })
    .then(response => {
        console.log('[confirmWithdrawal] API response received. Status:', response.status);
        console.log('[confirmWithdrawal] Response headers:', response.headers);
        
        if (!response.ok) {
            return response.json().then(data => {
                console.error('[confirmWithdrawal] Error response data:', data);
                throw new Error(data.error || 'Failed to process withdrawal');
            });
        }
        return response.json();
    })
    .then(data => {
        console.log('[confirmWithdrawal] API response data:', data);
        showStatus('Withdrawal transaction submitted! Waiting for blockchain confirmation...', 'info');
        
        // Close the modal
        document.getElementById('withdrawModal').style.display = 'none';
        
        // Start polling for blockchain state changes
        waitForBlockchainUpdate(isFullWithdrawal, modalCurrentStake, finalAmount);
    })
    .catch(error => {
        console.error('[confirmWithdrawal] Error:', error);
        showStatus(`Error: ${error.message}`, 'error');
        
        // Re-enable the button
        if (confirmButton) {
            confirmButton.disabled = false;
            confirmButton.textContent = 'Confirm Withdraw';
        }
    });
};

// Function to close any modal
function closeModal(modalId) {
    document.getElementById(modalId).style.display = 'none';
};

// Function to wait for blockchain state to update
function waitForBlockchainUpdate(isFullWithdrawal, originalStake, withdrawalAmount) {
    let stateAttempts = 0;
    const maxStateAttempts = 60; // 5 minutes (5s intervals)
    let lastStakeValue = null;
    let stableReadingCount = 0;
    
    showStatus('Waiting for blockchain state to update...', 'info');
    
    const checkStateStatus = setInterval(() => {
        stateAttempts++;
        console.log(`Checking contract state, attempt ${stateAttempts}`);
        
        if (stateAttempts % 4 === 0) {
            showStatus(`Waiting for blockchain state to update... (${Math.floor(stateAttempts / 12)} min)`, 'info');
        }
        
        // Check current blockchain state
        fetch('/api/status')
            .then(response => {
                if (!response.ok) {
                    throw new Error('Failed to check blockchain state');
                }
                return response.json();
            })
            .then(statusData => {
                console.log('Blockchain state:', statusData);
                
                // Parse the current stake from the status response
                const currentStakeFromStatus = parseFloat(statusData.verifierStake || "0");
                console.log('Current stake from status:', currentStakeFromStatus);
                
                if (isFullWithdrawal) {
                    // For full withdrawal, check if registration status has changed
                    if (!statusData.isRegistered || Math.abs(currentStakeFromStatus) < 0.000001) {
                        clearInterval(checkStateStatus);
                        showStatus('Withdrawal complete! You are no longer registered as a verifier.', 'success');
                        
                        // Refresh the page after a short delay
                        setTimeout(() => {
                            window.location.reload();
                        }, 3000);
                    }
                } else {
                    // For partial withdrawal, check if stake amount has changed
                    const expectedStakeAfterWithdrawal = originalStake - withdrawalAmount;
                    console.log('Expected stake after withdrawal:', expectedStakeAfterWithdrawal);
                    
                    // Check if stake has changed to expected value (with small tolerance for floating point)
                    const isCloseToExpected = Math.abs(currentStakeFromStatus - expectedStakeAfterWithdrawal) < 0.000001;
                    const hasDecreased = currentStakeFromStatus < originalStake;
                    
                    console.log('Is close to expected:', isCloseToExpected);
                    console.log('Has decreased:', hasDecreased);
                    
                    if (isCloseToExpected || hasDecreased) {
                        if (lastStakeValue === null) {
                            lastStakeValue = currentStakeFromStatus;
                            console.log('First stable reading:', lastStakeValue);
                        } else if (Math.abs(lastStakeValue - currentStakeFromStatus) < 0.000001) {
                            stableReadingCount++;
                            console.log('Stable reading count:', stableReadingCount);
                            
                            // If we get enough stable readings, consider it confirmed
                            if (stableReadingCount >= 3) {
                                clearInterval(checkStateStatus);
                                showStatus(`Withdrawal complete! Your stake has been updated to ${currentStakeFromStatus.toFixed(6)} DXP.`, 'success');
                                
                                // Refresh the page after a short delay
                                setTimeout(() => {
                                    window.location.reload();
                                }, 3000);
                            }
                        } else {
                            // Reset if the value is fluctuating
                            lastStakeValue = currentStakeFromStatus;
                            stableReadingCount = 0;
                            console.log('Reset stable reading count, new value:', lastStakeValue);
                        }
                    } else if (stateAttempts >= maxStateAttempts) {
                        clearInterval(checkStateStatus);
                        showStatus('Blockchain state update is taking longer than expected. Please refresh manually.', 'info');
                    }
                }
            })
            .catch(error => {
                console.error('Error checking blockchain state:', error);
                // Don't clear interval, keep checking
            });
    }, 5000); // Check every 5 seconds
}

function showStatus(message, type) {
    if (!actionStatus) return;
    
    actionStatus.textContent = message;
    actionStatus.classList.remove('hidden', 'success', 'error', 'info');
    actionStatus.classList.add(type);
    
    // Auto-hide after 5 seconds
    setTimeout(() => {
        actionStatus.classList.add('hidden');
    }, 5000);
}

// Function to toggle node status
function toggleNode() {
    if (!toggleNodeBtn) return;
    
    const isStarting = toggleNodeBtn.classList.contains('start-btn');
    const endpoint = isStarting ? '/api/start-node' : '/api/stop-node';
    
    // Disable button during operation
    toggleNodeBtn.disabled = true;
    
    fetch(endpoint, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(data => {
                throw new Error(data.error || 'Failed to toggle node');
            });
        }
        return response.json();
    })
    .then(data => {
        console.log('Node toggle success:', data);
        
        if (isStarting) {
            // Update UI for running node
            toggleNodeBtn.textContent = 'Stop Node';
            toggleNodeBtn.classList.remove('start-btn');
            toggleNodeBtn.classList.add('stop-btn');
            nodeStatusIndicator.textContent = 'Node Active';
            nodeStatusIndicator.classList.remove('inactive');
            nodeStatusIndicator.classList.add('active');
            terminalContainer.classList.remove('hidden');
            
            // Start polling for node output
            startNodeOutputPolling();
        } else {
            // Update UI for stopped node
            toggleNodeBtn.textContent = 'Start Node';
            toggleNodeBtn.classList.remove('stop-btn');
            toggleNodeBtn.classList.add('start-btn');
            nodeStatusIndicator.textContent = 'Node Inactive';
            nodeStatusIndicator.classList.remove('active');
            nodeStatusIndicator.classList.add('inactive');
            
            // Stop polling for node output
            if (nodeOutputPollInterval) {
                clearInterval(nodeOutputPollInterval);
            }
        }
        
        // Re-enable button
        toggleNodeBtn.disabled = false;
    })
    .catch(error => {
        console.error('Node toggle error:', error);
        showStatus(`Error: ${error.message}`, 'error');
        
        // Re-enable button
        toggleNodeBtn.disabled = false;
    });
}

// Function to poll for node output
function startNodeOutputPolling() {
    // Clear any existing interval
    if (nodeOutputPollInterval) {
        clearInterval(nodeOutputPollInterval);
    }
    
    // Function to fetch node output
    function fetchNodeOutput() {
        fetch('/api/node-output')
            .then(response => response.json())
            .then(data => {
                if (terminalOutput) {
                    // Update terminal output
                    terminalOutput.innerHTML = '';
                    
                    // Process and format the output lines
                    data.output.forEach(line => {
                        const formattedLine = document.createElement('div');
                        
                        // Apply styling based on line content
                        if (line.startsWith('✅')) {
                            formattedLine.classList.add('success');
                        } else if (line.startsWith('[ERROR]')) {
                            formattedLine.classList.add('error');
                        } else if (line.startsWith('[INFO]')) {
                            formattedLine.classList.add('info');
                        }
                        
                        formattedLine.textContent = line;
                        terminalOutput.appendChild(formattedLine);
                    });
                    
                    // Scroll to bottom
                    terminalOutput.scrollTop = terminalOutput.scrollHeight;
                    
                    // If node is no longer running, update UI
                    if (!data.running && nodeStatusIndicator && nodeStatusIndicator.classList.contains('active')) {
                        toggleNodeBtn.textContent = 'Start Node';
                        toggleNodeBtn.classList.remove('stop-btn');
                        toggleNodeBtn.classList.add('start-btn');
                        nodeStatusIndicator.textContent = 'Node Inactive';
                        nodeStatusIndicator.classList.remove('active');
                        nodeStatusIndicator.classList.add('inactive');
                        
                        // Stop polling
                        clearInterval(nodeOutputPollInterval);
                    }
                }
            })
            .catch(error => {
                console.error('Error fetching node output:', error);
            });
    }
    
    // Fetch output immediately
    fetchNodeOutput();
    
    // Then start polling every 1 second
    nodeOutputPollInterval = setInterval(fetchNodeOutput, 1000);
}

// Make the registerVerifier function globally accessible
window.registerVerifier = function() {
    const registerAmount = document.getElementById('register-amount');
    const registerFarmId = document.getElementById('register-farmid');
    const registerBtn = document.getElementById('register-btn');
    const registerStatus = document.getElementById('register-status');
    
    // Validate inputs
    const amount = registerAmount.value.trim();
    const farmId = registerFarmId.value.trim();
    
    if (!amount) {
        showRegisterStatus('Please enter a stake amount', 'error');
        return;
    }
    
    if (!farmId) {
        showRegisterStatus('Please enter a farm ID', 'error');
        return;
    }
    
    // Validate the amount is a valid number
    if (isNaN(parseFloat(amount)) || !isFinite(amount) || parseFloat(amount) <= 0) {
        showRegisterStatus('Please enter a valid positive number for stake amount', 'error');
        return;
    }
    
    // Validate farmId is a positive integer
    if (isNaN(parseInt(farmId)) || parseInt(farmId) <= 0) {
        showRegisterStatus('Farm ID must be a positive integer', 'error');
        return;
    }
    
    // Disable the button to prevent multiple submissions
    registerBtn.disabled = true;
    registerBtn.classList.add('disabled');
    
    // Show loading status
    showRegisterStatus('Initiating registration process...', 'info');
    
    // Create URL-encoded form data
    const formData = new URLSearchParams();
    formData.append('amount', amount);
    formData.append('farmId', farmId);
    
    console.log('Submitting registration request for amount:', amount, 'farmId:', farmId);
    
    fetch('/api/register', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
        },
        body: formData.toString()
    })
    .then(response => {
        console.log('Registration response status:', response.status);
        if (!response.ok) {
            return response.json().then(data => {
                console.error('Registration error:', data);
                throw new Error(data.error || 'Failed to process registration');
            });
        }
        return response.json();
    })
    .then(data => {
        console.log('Registration process initiated:', data);
        
        if (data.step === 'approval') {
            // Step 1: Token approval submitted
            showRegisterStatus(`Token approval submitted (${data.approvalTxHash.substring(0, 10)}...). Waiting for confirmation...`, 'info');
            
            // Set up polling to check registration status
            let attempts = 0;
            const maxAttempts = 60; // 5 minutes (5s intervals)
            const statusCheckInterval = setInterval(() => {
                attempts++;
                
                // Check if the user is now registered
                fetch('/api/status')
                    .then(response => response.json())
                    .then(statusData => {
                        console.log('Status check attempt', attempts, statusData);
                        
                        if (statusData.isRegistered) {
                            // Registration complete!
                            clearInterval(statusCheckInterval);
                            showRegisterStatus('Successfully registered as a verifier!', 'success');
                            
                            // Refresh the page after a short delay
                            setTimeout(() => {
                                window.location.reload();
                            }, 3000);
                        } else if (attempts >= maxAttempts) {
                            clearInterval(statusCheckInterval);
                            showRegisterStatus('Registration is taking longer than expected. Please check status later or try again.', 'info');
                            
                            // Re-enable the button
                            registerBtn.disabled = false;
                            registerBtn.classList.remove('disabled');
                        } else {
                            // Still processing
                            const message = attempts % 4 === 0 ? 
                                'Registration in progress. This may take a few minutes...' : 
                                showRegisterStatus.textContent;
                                
                            if (attempts % 4 === 0) {
                                showRegisterStatus(message, 'info');
                            }
                        }
                    })
                    .catch(error => {
                        console.error('Status check error:', error);
                        // Don't clear interval, keep trying
                    });
            }, 5000); // Check every 5 seconds
        } else {
            // Direct registration response (should not happen with our implementation)
            showRegisterStatus(`Registration submitted: ${data.status}`, 'success');
            
            // Refresh the page after a delay
            setTimeout(() => {
                window.location.reload();
            }, 10000);
        }
    })
    .catch(error => {
        console.error('Registration error:', error);
        showRegisterStatus(`Error: ${error.message}`, 'error');
        
        // Re-enable the button on error
        registerBtn.disabled = false;
        registerBtn.classList.remove('disabled');
    });
};

function showRegisterStatus(message, type) {
    const registerStatus = document.getElementById('register-status');
    if (!registerStatus) return;
    
    registerStatus.textContent = message;
    registerStatus.classList.remove('hidden', 'success', 'error', 'info');
    registerStatus.classList.add(type);
    
    // Auto-hide after 5 seconds for success/error messages
    if (type !== 'info') {
        setTimeout(() => {
            registerStatus.classList.add('hidden');
        }, 5000);
    }
}

// Initialize the verification chart with dummy data
function initVerificationChart() {
    const ctx = document.getElementById('verificationsChart');
    if (!ctx) return;

    // Generate dummy data for the past 7 days
    const labels = [];
    const data = [];
    const now = new Date();
    
    for (let i = 6; i >= 0; i--) {
        const date = new Date(now);
        date.setDate(date.getDate() - i);
        labels.push(date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }));
        
        // Generate random verification count between 3 and 8
        const count = Math.floor(Math.random() * 6) + 3;
        data.push(count);
    }

    // Add today's actual count if available
    const verificationsEl = document.querySelector('.metric:first-child .value');
    if (verificationsEl) {
        const actualCount = parseInt(verificationsEl.textContent.trim(), 10);
        if (!isNaN(actualCount)) {
            data[data.length - 1] = actualCount;
        }
    }

    // Create the chart
    new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: 'Verifications',
                data: data,
                borderColor: '#4a8af4',
                backgroundColor: 'rgba(74, 138, 244, 0.1)',
                borderWidth: 2,
                tension: 0.3,
                fill: true,
                pointBackgroundColor: '#4a8af4',
                pointRadius: 4,
                pointHoverRadius: 6
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    backgroundColor: '#1e2130',
                    titleColor: '#fff',
                    bodyColor: '#fff',
                    borderColor: '#4a8af4',
                    borderWidth: 1,
                    displayColors: false,
                    callbacks: {
                        title: function(tooltipItems) {
                            return tooltipItems[0].label;
                        },
                        label: function(context) {
                            return `Verifications: ${context.raw}`;
                        }
                    }
                }
            },
            scales: {
                x: {
                    grid: {
                        color: 'rgba(255, 255, 255, 0.05)'
                    },
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.7)'
                    }
                },
                y: {
                    beginAtZero: true,
                    grid: {
                        color: 'rgba(255, 255, 255, 0.05)'
                    },
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.7)',
                        precision: 0
                    }
                }
            }
        }
    });
}
