// Global variables
let pendingRewardsEl;
let claimRewardsBtn;
let confirmWithdrawBtn;
let withdrawAmount;
let actionStatus;
let refreshBtn;

document.addEventListener('DOMContentLoaded', function() {
    // Elements
    pendingRewardsEl = document.getElementById('pending-rewards');
    claimRewardsBtn = document.getElementById('claim-rewards-btn');
    confirmWithdrawBtn = document.getElementById('confirm-withdraw-btn');
    withdrawAmount = document.getElementById('withdraw-amount');
    actionStatus = document.getElementById('action-status');
    refreshBtn = document.getElementById('refresh-btn');

    // Debug element references
    console.log('Elements loaded:', {
        pendingRewardsEl: pendingRewardsEl,
        claimRewardsBtn: claimRewardsBtn,
        confirmWithdrawBtn: confirmWithdrawBtn,
        withdrawAmount: withdrawAmount,
        actionStatus: actionStatus,
        refreshBtn: refreshBtn
    });

    // Fetch initial status
    fetchStatus();

    // Set up event listeners
    if (claimRewardsBtn) {
        claimRewardsBtn.addEventListener('click', claimRewards);
        console.log('Claim rewards button listener attached');
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
    console.log('withdrawStake function called');
    const amount = withdrawAmount.value.trim();
    if (!amount) {
        showStatus('Please enter an amount to withdraw', 'error');
        return;
    }

    // Validate the amount is a valid number
    if (isNaN(parseFloat(amount)) || !isFinite(amount) || parseFloat(amount) <= 0) {
        showStatus('Please enter a valid positive number', 'error');
        return;
    }

    // Disable the button to prevent multiple submissions
    if (confirmWithdrawBtn) {
        confirmWithdrawBtn.disabled = true;
        confirmWithdrawBtn.classList.add('disabled');
    }
    
    // Show loading status
    showStatus('Processing withdrawal...', 'info');
    
    // Create URL-encoded form data instead of FormData
    const formData = new URLSearchParams();
    formData.append('amount', amount);

    console.log('Submitting withdrawal request for amount:', amount);

    fetch('/api/withdraw', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
        },
        body: formData.toString()
    })
    .then(response => {
        console.log('Withdrawal response status:', response.status);
        if (!response.ok) {
            return response.json().then(data => {
                console.error('Withdrawal error:', data);
                throw new Error(data.error || 'Failed to process withdrawal');
            });
        }
        return response.json();
    })
    .then(data => {
        console.log('Withdrawal success:', data);
        showStatus(`Success: ${data.status}. Transaction: ${data.txHash}`, 'success');
        withdrawAmount.value = '';
        
        // Add transaction waiting message
        setTimeout(() => {
            showStatus('Waiting for transaction confirmation...', 'info');
        }, 3000);
        
        // Set up polling to check registration status
        let attempts = 0;
        const maxAttempts = 30; // 30 seconds (1s intervals)
        const statusCheckInterval = setInterval(() => {
            attempts++;
            
            // Check if the user is now unregistered
            fetch('/api/status')
                .then(response => response.json())
                .then(statusData => {
                    console.log('Status check attempt', attempts, statusData);
                    
                    if (!statusData.isRegistered) {
                        // Successfully unregistered!
                        clearInterval(statusCheckInterval);
                        showStatus('Withdrawal confirmed! Refreshing page...', 'success');
                        
                        // Refresh the page after a short delay
                        setTimeout(() => {
                            window.location.reload();
                        }, 2000);
                    } else if (attempts >= maxAttempts) {
                        // Timeout after max attempts, force refresh anyway
                        clearInterval(statusCheckInterval);
                        showStatus('Status update taking longer than expected. Refreshing page...', 'info');
                        
                        // Force refresh after timeout
                        setTimeout(() => {
                            window.location.reload();
                        }, 2000);
                    }
                })
                .catch(error => {
                    console.error('Status check error:', error);
                    // Don't clear interval, keep trying
                });
        }, 1000); // Check every second
    })
    .catch(error => {
        console.error('Withdrawal error:', error);
        showStatus(`Error: ${error.message}`, 'error');
        
        // Re-enable the button on error
        if (confirmWithdrawBtn) {
            confirmWithdrawBtn.disabled = false;
            confirmWithdrawBtn.classList.remove('disabled');
        }
    });
};

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
                            // Timeout after max attempts
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
