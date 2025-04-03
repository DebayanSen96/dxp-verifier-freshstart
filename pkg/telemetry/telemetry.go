package telemetry

import (
	"runtime"
	"time"
)

// Metrics represents telemetry data collected during farm score calculation
type Metrics struct {
	// Processing time in milliseconds
	ProcessingTime int64 `json:"processing_time"`
	
	// Memory usage in bytes
	MemoryUsage uint64 `json:"memory_usage"`
	
	// CPU usage percentage (0-100)
	CPUUsage float64 `json:"cpu_usage"`
	
	// Number of calculations performed
	CalculationCount int `json:"calculation_count"`
	
	// Timestamp when metrics were collected
	Timestamp int64 `json:"timestamp"`
	
	// Additional metrics can be added here
	DataSize int `json:"data_size"`
}

// CollectMetrics collects system metrics
func CollectMetrics() *Metrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return &Metrics{
		MemoryUsage:     m.Alloc,
		Timestamp:       time.Now().UnixNano() / int64(time.Millisecond),
		CPUUsage:        getCPUUsage(),
	}
}

// StartTimer starts a timer for measuring processing time
func StartTimer() time.Time {
	return time.Now()
}

// StopTimer stops the timer and returns the elapsed time in milliseconds
func StopTimer(startTime time.Time) int64 {
	return time.Since(startTime).Milliseconds()
}

// getCPUUsage gets an approximation of CPU usage
// In a real implementation, this would use OS-specific APIs
func getCPUUsage() float64 {
	// This is a simplified implementation
	// In a real system, you would use OS-specific APIs or libraries like gopsutil
	var startStat, endStat runtime.MemStats
	
	runtime.ReadMemStats(&startStat)
	
	// Do some work to measure CPU
	for i := 0; i < 1000000; i++ {
		_ = i * i
	}
	
	runtime.ReadMemStats(&endStat)
	
	// This is not a real CPU measurement, just an approximation for demonstration
	return float64(endStat.TotalAlloc-startStat.TotalAlloc) / 1024.0
}
