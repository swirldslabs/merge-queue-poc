package sniff

type MemStats struct {
	AllocMiB      uint64 `json:"alloc_mib"`
	TotalAllocMiB uint64 `json:"total_alloc_mib"`
	SysMiB        uint64 `json:"sys_mib"`
	NumGC         uint32 `json:"num_gc"`
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
	Enabled      bool
	Interval     string
	Directory    string
	EnableServer bool
	// ServerHost is the Host for the pprof server.
	ServerHost string
	// ServerPort is the port for the pprof server.
	ServerPort int
	// MaxSize is the maximum size (in MB) of a stats file before it is rolled.
	MaxSize int64
}
