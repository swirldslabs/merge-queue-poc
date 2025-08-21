package sniff

type MemStats struct {
	AllocMiB        uint64 `json:"alloc_mib"`
	HeapAllocMiB    uint64 `json:"heap_alloc_mib"`
	HeapSysMiB      uint64 `json:"heap_sys_mib"`
	HeapIdleMiB     uint64 `json:"heap_idle_mib"`
	HeapInuseMiB    uint64 `json:"heap_in_use_mib"`
	HeapReleasedMiB uint64 `json:"heap_released_mib"`
	HeapObjects     uint64 `json:"heap_objects"`
	Mallocs         uint64 `json:"mallocs"`
	Frees           uint64 `json:"frees"`
	LiveObjects     uint64 `json:"live_objects"`
	SysMiB          uint64 `json:"sys_mib"`
	NumGC           uint32 `json:"num_gc"`
}

type CPUStats struct {
	NumGoroutines int   `json:"num_goroutines"`
	NumCPU        int   `json:"num_cpu"`
	NumCgoCalls   int64 `json:"num_cgo_calls"`
}

type Stats struct {
	Pid       int       `json:"pid"`
	Timestamp string    `json:"timestamp"`
	MemStats  *MemStats `json:"mem_stats"`
	CPUStats  *CPUStats `json:"cpu_stats"`
}

type ProfilingConfig struct {
	// Enabled indicates whether profiling is enabled.
	Enabled bool
	// Interval is the profiling interval in a string format, e.g., "1s", "10m".
	Interval string
	// ServerHost is the Host for the profiling server.
	ServerHost string
	// ServerPort is the port for the profiling server.
	ServerPort int
	// EnablePprofServer indicates whether to enable the pprof server.
	EnablePprofServer bool
	// PprofPort is the port for the pprof server.
	PprofPort int
	// FileLogging indicates whether to enable stats being written to file.
	FileLogging bool
	// Directory is the directory where stats files will be stored.
	// It creates a file stats.json in the specified directory.
	Directory string
	// MaxSize is the maximum size (in MB) of a stats file before it is rolled.
	MaxSize int64
}
