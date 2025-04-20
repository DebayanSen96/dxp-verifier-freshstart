package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	// LogLevelDebug represents debug level logs
	LogLevelDebug = "DEBUG"
	// LogLevelInfo represents info level logs
	LogLevelInfo = "INFO"
	// LogLevelWarn represents warning level logs
	LogLevelWarn = "WARN"
	// LogLevelError represents error level logs
	LogLevelError = "ERROR"
)

var (
	// Default logger
	defaultLogger *Logger
	// Log file
	logFile *os.File
)

// Logger represents a logger instance
type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	console     bool
	// Controls whether INFO level logs are printed to console
	verboseConsole bool
}

// Init initializes the default logger
func Init(logDir string, console bool) error {
	return InitWithVerbosity(logDir, console, false)
}

// InitWithVerbosity initializes the default logger with control over verbose console output
func InitWithVerbosity(logDir string, console bool, verboseConsole bool) error {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Create log file with current date
	logFileName := fmt.Sprintf("dxp-verifier-%s.log", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(logDir, logFileName)
	
	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Create default logger
	defaultLogger = &Logger{
		debugLogger: log.New(logFile, "[DEBUG] ", log.Ldate|log.Ltime),
		infoLogger:  log.New(logFile, "[INFO] ", log.Ldate|log.Ltime),
		warnLogger:  log.New(logFile, "[WARN] ", log.Ldate|log.Ltime),
		errorLogger: log.New(logFile, "[ERROR] ", log.Ldate|log.Ltime),
		console:     console,
		verboseConsole: verboseConsole,
	}

	return nil
}

// Close closes the log file
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	if defaultLogger == nil {
		return
	}
	
	msg := fmt.Sprintf(format, v...)
	defaultLogger.debugLogger.Println(msg)
	
	if defaultLogger.console {
		fmt.Printf("[DEBUG] %s\n", msg)
	}
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	if defaultLogger == nil {
		return
	}
	
	msg := fmt.Sprintf(format, v...)
	defaultLogger.infoLogger.Println(msg)
	
	// Only print to console if verbose console logging is enabled
	if defaultLogger.console && defaultLogger.verboseConsole {
		fmt.Printf("[INFO] %s\n", msg)
	}
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	if defaultLogger == nil {
		return
	}
	
	msg := fmt.Sprintf(format, v...)
	defaultLogger.warnLogger.Println(msg)
	
	if defaultLogger.console {
		fmt.Printf("[WARN] %s\n", msg)
	}
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	if defaultLogger == nil {
		return
	}
	
	msg := fmt.Sprintf(format, v...)
	defaultLogger.errorLogger.Println(msg)
	
	if defaultLogger.console {
		fmt.Printf("[ERROR] %s\n", msg)
	}
}

// Console logs a message only to the console (not to the log file)
func Console(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

// Success logs a success message to the console with a checkmark
func Success(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	
	if defaultLogger != nil {
		defaultLogger.infoLogger.Println(msg)
	}
	
	fmt.Printf("✅ "+format+"\n", v...)
}

// InfoIcon logs an info message to the console with an info icon
func InfoIcon(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	
	if defaultLogger != nil {
		defaultLogger.infoLogger.Println(msg)
	}
	
	fmt.Printf("ℹ️ "+format+"\n", v...)
}

// LogOnly logs a message only to the log file (not to the console)
func LogOnly(format string, v ...interface{}) {
	if defaultLogger == nil {
		return
	}
	
	msg := fmt.Sprintf(format, v...)
	defaultLogger.infoLogger.Println(msg)
}
