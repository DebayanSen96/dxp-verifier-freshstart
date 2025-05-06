package dashboard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/dexponent/dxp-verifier/pkg/eth"
	"github.com/dexponent/dxp-verifier/pkg/logger"
)

// Server represents the dashboard web server
type Server struct {
	ethClient     *eth.Client
	templates     *template.Template
	port          string
	nodeCmd       *exec.Cmd
	nodeMutex     sync.Mutex
	nodeOutput    []string
	isNodeRunning bool
}

// NewServer creates a new dashboard server
func NewServer(ethClient *eth.Client) *Server {
	return &Server{
		ethClient: ethClient,
		templates: template.Must(template.ParseGlob("pkg/dashboard/templates/*.html")),
		port:      "8080",
	}
}

// Start starts the dashboard server
func (s *Server) Start() error {
	// Create router
	mux := http.NewServeMux()

	// Serve static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("pkg/dashboard/static"))))

	// API endpoints
	mux.HandleFunc("/api/status", s.handleStatusAPI)
	mux.HandleFunc("/api/claim-rewards", s.handleClaimRewardsAPI)
	mux.HandleFunc("/api/withdraw", s.handleWithdrawAPI)
	mux.HandleFunc("/api/register", s.handleRegisterAPI)
	mux.HandleFunc("/api/start-node", s.handleStartNodeAPI)
	mux.HandleFunc("/api/stop-node", s.handleStopNodeAPI)
	mux.HandleFunc("/api/node-output", s.handleNodeOutputAPI)
	mux.HandleFunc("/api/transaction-status", s.handleTransactionStatusAPI)

	// Main page
	mux.HandleFunc("/", s.handleIndex)

	// Start server
	addr := ":" + s.port
	logger.Info("Starting dashboard on http://localhost%s", addr)

	// Open browser
	go s.openBrowser("http://localhost" + addr)

	return http.ListenAndServe(addr, mux)
}

// handleIndex renders the main dashboard page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get verifier data
	isRegistered, err := s.ethClient.IsRegisteredVerifier()
	if err != nil {
		logger.Error("Failed to check if verifier is registered: %v", err)
	}

	var pendingRewards string
	var verifierStake string
	var assignedFarms []int64

	if isRegistered {
		// Get pending rewards
		rewards, err := s.ethClient.CalculatePendingRewards()
		if err == nil {
			pendingRewards = s.ethClient.FormatTokenAmount(rewards)
		}

		// Get verifier stake
		stake, err := s.ethClient.GetVerifierStake(1)
		if err == nil {
			verifierStake = s.ethClient.FormatTokenAmount(stake)
		}

		// Get assigned farms
		assignedFarms, _ = s.ethClient.GetAssignedFarms()
	}

	// Get wallet address
	walletAddress := s.ethClient.GetWalletAddress()

	// Get token balance
	tokenBalance, err := s.ethClient.GetDXPBalance()
	if err != nil {
		logger.Error("Failed to get token balance: %v", err)
	}

	// Prepare data for template
	data := map[string]interface{}{
		"WalletAddress":  walletAddress,
		"IsRegistered":   isRegistered,
		"TokenBalance":   s.ethClient.FormatTokenAmount(tokenBalance),
		"PendingRewards": pendingRewards,
		"VerifierStake":  verifierStake,
		"AssignedFarms":  assignedFarms,
		"CurrentTime":    time.Now().Format(time.RFC1123),
	}

	// Render template
	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		logger.Error("Failed to render template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// formatTimestamp formats a Unix timestamp as a human-readable date/time
func formatTimestamp(timestamp uint64) string {
	if timestamp == 0 {
		return "Never"
	}
	t := time.Unix(int64(timestamp), 0)
	return t.Format(time.RFC1123)
}

// formatDuration formats seconds as a human-readable duration
func formatDuration(seconds uint64) string {
	if seconds == 0 {
		return "0s"
	}

	duration := time.Duration(seconds) * time.Second

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// API Handlers

// handleStatusAPI returns the current verifier status as JSON
func (s *Server) handleStatusAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	isRegistered, err := s.ethClient.IsRegisteredVerifier()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}

	var pendingRewards string
	var verifierStake string

	if isRegistered {
		// Get verifier stake
		stake, err := s.ethClient.GetVerifierStake(1)
		if err == nil {
			verifierStake = s.ethClient.FormatTokenAmount(stake)
		}

		// Get pending rewards
		rewards, err := s.ethClient.CalculatePendingRewards()
		if err == nil {
			pendingRewards = s.ethClient.FormatTokenAmount(rewards)
		}
	}

	// Prepare response
	assignedFarms, _ := s.ethClient.GetAssignedFarms()
	response := map[string]interface{}{
		"isRegistered":   isRegistered,
		"walletAddress":  s.ethClient.GetWalletAddress(),
		"pendingRewards": pendingRewards,
		"verifierStake":  verifierStake,
		"assignedFarms":  assignedFarms,
	}

	json.NewEncoder(w).Encode(response)
}

// handleClaimRewardsAPI handles the claim rewards action
func (s *Server) handleClaimRewardsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Claim rewards
	tx, err := s.ethClient.ClaimRewards()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}

	// Return transaction hash
	response := map[string]string{
		"txHash": tx.Hash().Hex(),
		"status": "Rewards claim transaction submitted",
	}

	json.NewEncoder(w).Encode(response)
}

// handleWithdrawAPI handles the withdraw stake action
func (s *Server) handleWithdrawAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		logger.Error("Failed to parse form data: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusBadRequest)
		return
	}

	amountStr := r.FormValue("amount")
	if amountStr == "" {
		logger.Error("Amount is required")
		http.Error(w, `{"error": "Amount is required"}`, http.StatusBadRequest)
		return
	}

	logger.Info("Withdraw request received for amount: %s", amountStr)

	// Check if registered as verifier
	isRegistered, err := s.ethClient.IsRegisteredVerifier()
	if err != nil {
		logger.Error("Failed to check verifier status: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to check verifier status: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if !isRegistered {
		logger.Error("Not registered as a verifier")
		http.Error(w, `{"error": "Not registered as a verifier"}`, http.StatusBadRequest)
		return
	}

	// Get verifier stake
	stake, err := s.ethClient.GetVerifierStake(1)
	if err != nil {
		logger.Error("Failed to get verifier stake: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to get verifier stake: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Convert amount string to wei
	amountWei, err := s.ethClient.ConvertToWei(amountStr)
	if err != nil {
		logger.Error("Invalid amount: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Invalid amount: %v"}`, err), http.StatusBadRequest)
		return
	}

	// Check if stake is sufficient
	if stake.Cmp(amountWei) < 0 {
		errorMsg := fmt.Sprintf("Insufficient stake. Requested: %s DXP, Available: %s DXP",
			s.ethClient.FormatTokenAmount(amountWei),
			s.ethClient.FormatTokenAmount(stake))
		logger.Error("Validation failed: %s", errorMsg)
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, errorMsg), http.StatusBadRequest)
		return
	}

	// Define minimum stake requirement (100 DXP)
	minStakeWei, _ := s.ethClient.ConvertToWei("100")

	// Calculate remaining stake after withdrawal
	remainingStake := new(big.Int).Sub(stake, amountWei)

	// Check if remaining stake would be below minimum but greater than zero
	if remainingStake.Cmp(big.NewInt(0)) > 0 && remainingStake.Cmp(minStakeWei) < 0 {
		errorMsg := fmt.Sprintf("Withdrawal would leave stake below minimum requirement of 100 DXP. Requested: %s DXP, Remaining would be: %s DXP",
			s.ethClient.FormatTokenAmount(amountWei),
			s.ethClient.FormatTokenAmount(remainingStake))
		logger.Error("Validation failed: %s", errorMsg)
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, errorMsg), http.StatusBadRequest)
		return
	}

	logger.Info("Withdrawing %s DXP from stake...", s.ethClient.FormatTokenAmount(amountWei))

	// Get farmID from request, default to 1 if not provided
	farmID := int64(1)
	farmIDStr := r.FormValue("farmId")
	if farmIDStr != "" {
		farmIDInt, err := strconv.ParseInt(farmIDStr, 10, 64)
		if err == nil && farmIDInt > 0 {
			farmID = farmIDInt
		}
	}

	logger.Info("Withdrawing %s DXP from farm ID %d...", s.ethClient.FormatTokenAmount(amountWei), farmID)

	// Withdraw stake
	tx, err := s.ethClient.WithdrawVerifierStake(farmID, amountWei)
	if err != nil {
		logger.Error("Failed to withdraw stake: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to withdraw stake: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Return transaction hash
	txHash := tx.Hash().Hex()
	logger.Success("Withdrawal transaction submitted: %s", txHash)

	response := map[string]string{
		"txHash": txHash,
		"status": "Withdrawal transaction submitted",
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleRegisterAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		logger.Error("Failed to parse form data: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusBadRequest)
		return
	}

	// Get amount and farmId from form
	amountStr := r.FormValue("amount")
	farmIdStr := r.FormValue("farmId")

	if amountStr == "" {
		logger.Error("Amount is required")
		http.Error(w, `{"error": "Amount is required"}`, http.StatusBadRequest)
		return
	}

	if farmIdStr == "" {
		logger.Error("Farm ID is required")
		http.Error(w, `{"error": "Farm ID is required"}`, http.StatusBadRequest)
		return
	}

	// Convert amount to wei
	amountWei, err := s.ethClient.ConvertToWei(amountStr)
	if err != nil {
		logger.Error("Invalid amount: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Invalid amount: %v"}`, err), http.StatusBadRequest)
		return
	}

	// Convert farmId to int64
	farmId, err := strconv.ParseInt(farmIdStr, 10, 64)
	if err != nil {
		logger.Error("Invalid farm ID: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Invalid farm ID: %v"}`, err), http.StatusBadRequest)
		return
	}

	// Check if already registered
	isRegistered, err := s.ethClient.IsRegisteredVerifier()
	if err != nil {
		logger.Error("Failed to check verifier status: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to check verifier status: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if isRegistered {
		logger.Error("Already registered as a verifier")
		http.Error(w, `{"error": "Already registered as a verifier"}`, http.StatusBadRequest)
		return
	}

	// Step 1: Approve DXP token transfer
	logger.Info("Approving DXP token transfer...")
	approvalTxHash, err := s.ethClient.ApproveDXPToken(amountWei)
	if err != nil {
		logger.Error("Failed to approve DXP token transfer: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to approve DXP token transfer: %v"}`, err), http.StatusInternalServerError)
		return
	}

	logger.Success("Approval transaction submitted: %s", approvalTxHash)

	// Return the approval transaction hash immediately so the UI can show progress
	response := map[string]interface{}{
		"approvalTxHash": approvalTxHash,
		"status":         "Approval transaction submitted. Registration will be processed after approval is confirmed.",
		"step":           "approval",
	}

	// Start a goroutine to handle the rest of the registration process
	go func() {
		// Wait for the approval transaction to be mined
		logger.Info("Waiting for approval transaction to be mined...")
		_, err = s.ethClient.WaitForTransaction(approvalTxHash)
		if err != nil {
			logger.Error("Failed to wait for approval transaction: %v", err)
			return
		}

		logger.Success("Approval transaction mined successfully")

		// Step 2: Register as a verifier with the specified farm ID
		logger.Info("Registering verifier with amount %s and farm ID %d", s.ethClient.FormatTokenAmount(amountWei), farmId)
		tx, err := s.ethClient.RegisterVerifierWithFarmID(amountWei, farmId)
		if err != nil {
			logger.Error("Failed to register verifier: %v", err)
			return
		}

		txHash := tx.Hash().Hex()
		logger.Success("Registration transaction submitted: %s", txHash)

		// Wait for the registration transaction to be mined
		logger.Info("Waiting for registration transaction to be mined...")
		_, err = s.ethClient.WaitForTransaction(txHash)
		if err != nil {
			logger.Error("Failed to wait for registration transaction: %v", err)
			return
		}

		logger.Success("Successfully registered as a verifier!")
		logger.Info("Farm ID: %d", farmId)
		logger.Info("Staked Amount: %s", s.ethClient.FormatTokenAmount(amountWei))
	}()

	json.NewEncoder(w).Encode(response)
}

// openBrowser opens the default browser to the specified URL
func (s *Server) openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		logger.Error("Unsupported platform")
		return
	}

	if err != nil {
		logger.Error("Failed to open browser: %v", err)
	}
}

// StartDashboard initializes and starts the dashboard
func StartDashboard(ethClient *eth.Client) error {
	server := NewServer(ethClient)
	return server.Start()
}

// handleStartNodeAPI starts the verifier node
func (s *Server) handleStartNodeAPI(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.nodeMutex.Lock()
	defer s.nodeMutex.Unlock()

	// Check if node is already running
	if s.isNodeRunning {
		http.Error(w, `{"error": "Node is already running"}`, http.StatusBadRequest)
		return
	}

	// Start the node process
	cmd := exec.Command("./dxp-verifier", "start")

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("Failed to create stdout pipe: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create stdout pipe: %v"}`, err), http.StatusInternalServerError)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Error("Failed to create stderr pipe: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create stderr pipe: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start node: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to start node: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Store the command
	s.nodeCmd = cmd
	s.isNodeRunning = true
	s.nodeOutput = []string{}

	// Start goroutines to read output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			s.nodeMutex.Lock()
			s.nodeOutput = append(s.nodeOutput, line)
			s.nodeMutex.Unlock()
			logger.Info("Node: %s", line)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			s.nodeMutex.Lock()
			s.nodeOutput = append(s.nodeOutput, line)
			s.nodeMutex.Unlock()
			logger.Error("Node: %s", line)
		}
	}()

	// Start a goroutine to wait for the process to finish
	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Error("Node process exited with error: %v", err)
		} else {
			logger.Info("Node process exited normally")
		}

		s.nodeMutex.Lock()
		s.isNodeRunning = false
		s.nodeMutex.Unlock()
	}()

	// Return success response
	response := map[string]interface{}{
		"status": "Node started successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// handleStopNodeAPI stops the verifier node
func (s *Server) handleStopNodeAPI(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.nodeMutex.Lock()
	defer s.nodeMutex.Unlock()

	// Check if node is running
	if !s.isNodeRunning || s.nodeCmd == nil || s.nodeCmd.Process == nil {
		http.Error(w, `{"error": "Node is not running"}`, http.StatusBadRequest)
		return
	}

	// Stop the node process
	if err := s.nodeCmd.Process.Signal(syscall.SIGTERM); err != nil {
		logger.Error("Failed to stop node: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to stop node: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Add a message to the output
	s.nodeOutput = append(s.nodeOutput, "[INFO] Stopping node...")

	// Return success response
	response := map[string]interface{}{
		"status": "Node stopping...",
	}
	json.NewEncoder(w).Encode(response)
}

// handleNodeOutputAPI returns the current node output
func (s *Server) handleNodeOutputAPI(w http.ResponseWriter, r *http.Request) {
	s.nodeMutex.Lock()
	defer s.nodeMutex.Unlock()

	// Return the current output
	response := map[string]interface{}{
		"output":  s.nodeOutput,
		"running": s.isNodeRunning,
	}
	json.NewEncoder(w).Encode(response)
}

// handleTransactionStatusAPI returns the status of a transaction
func (s *Server) handleTransactionStatusAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get transaction hash from query parameter
	txHash := r.URL.Query().Get("txHash")
	if txHash == "" {
		http.Error(w, `{"error": "Transaction hash is required"}`, http.StatusBadRequest)
		return
	}

	// Get transaction status
	status, err := s.ethClient.GetTransactionStatus(txHash)
	if err != nil {
		logger.Error("Failed to get transaction status: %s", fmt.Sprintf("%v", err))
		http.Error(w, `{"error": "Failed to get transaction status", "mined": false}`, http.StatusInternalServerError)
		return
	}

	// Return transaction status
	json.NewEncoder(w).Encode(status)
}
