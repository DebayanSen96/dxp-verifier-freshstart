package dashboard

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/dexponent/dxp-verifier/pkg/eth"
	"github.com/dexponent/dxp-verifier/pkg/logger"
)

// Server represents the dashboard web server
type Server struct {
	ethClient *eth.Client
	templates *template.Template
	port      string
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

	// Get metrics if registered
	var metrics *eth.VerifierMetrics
	var formattedMetrics map[string]interface{}
	var pendingRewards string
	var verifierStake string
	var assignedFarms []int64
	
	if isRegistered {
		metrics, err = s.ethClient.GetVerifierMetrics()
		if err != nil {
			logger.Error("Failed to get verifier metrics: %v", err)
		} else {
			// Format metrics for display
			formattedMetrics = map[string]interface{}{
				"VerificationsPerformed": metrics.VerificationsPerformed,
				"LastActiveTime":         formatTimestamp(metrics.LastActiveTimestamp),
				"TotalUptimeFormatted":   formatDuration(metrics.TotalUptime),
				"AccumulatedRewards":     s.ethClient.FormatTokenAmount(metrics.AccumulatedRewards),
			}
		}
		
		// Get pending rewards
		rewards, err := s.ethClient.CalculatePendingRewards()
		if err == nil {
			pendingRewards = s.ethClient.FormatTokenAmount(rewards)
		}
		
		// Get verifier stake
		stake, err := s.ethClient.GetVerifierStake()
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
		"Metrics":        formattedMetrics,
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

	var metrics *eth.VerifierMetrics
	var pendingRewards string
	var assignedFarms []int64

	if isRegistered {
		metrics, err = s.ethClient.GetVerifierMetrics()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}

		rewards, err := s.ethClient.CalculatePendingRewards()
		if err == nil {
			pendingRewards = s.ethClient.FormatTokenAmount(rewards)
		}

		assignedFarms, _ = s.ethClient.GetAssignedFarms()
	}

	// Prepare response
	response := map[string]interface{}{
		"isRegistered":   isRegistered,
		"walletAddress":  s.ethClient.GetWalletAddress(),
		"metrics":        metrics,
		"pendingRewards": pendingRewards,
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
	stake, err := s.ethClient.GetVerifierStake()
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
		logger.Error(errorMsg)
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, errorMsg), http.StatusBadRequest)
		return
	}

	logger.Info("Withdrawing %s DXP from stake...", s.ethClient.FormatTokenAmount(amountWei))

	// Withdraw stake
	tx, err := s.ethClient.WithdrawVerifierStake(amountWei)
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
		"status": "Approval transaction submitted. Registration will be processed after approval is confirmed.",
		"step": "approval",
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
