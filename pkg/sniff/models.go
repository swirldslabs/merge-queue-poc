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
	Enabled   bool
	Interval  string
	Directory string
	// MaxSize is the maximum size (in MB) of a stats file before it is rolled.
	// ServerHost is the Host for the pprof server.
	ServerHost string
	// ServerPort is the port for the pprof server.
	ServerPort int
	// MaxSize is the maximum size (in MB) of a stats file before it is rolled.
	MaxSize           int64
	EnablePprofServer bool
	PprofPort         int
}
