package core

import (
	"context"
	"os"
	"time"
)

// Scanner defines the interface for a file scanning component.
//
// Methods:
//   - Info: Returns a unique identifier or description of the scanner instance.
//   - Scan: Scans a given directory and streams the results through a channel.
//     Errors encountered during the scanning process are sent to an error channel.
//
// Notes:
//   - Implementations of this interface are responsible for traversing directories or files
//     and providing metadata about the scanned items.
//   - The `Scan` method should support context cancellation to allow graceful termination of the scanning process.
type Scanner interface {
	Info() string
	Scan(ctx context.Context, ech chan<- error) <-chan ScannerResult
}

// ScannerResult represents the result of scanning a file or directory.
//
// Fields:
//   - Path: The path of the file that was found during scan(e.g. marker file).
//   - Info: The file information (os.FileInfo) associated with the scanned file.
//
// Notes:
//   - This struct is used to communicate the details of a matched file during scan.
type ScannerResult struct {
	Path    string
	TraceId string // Unique identifier for tracing the file processing
	Info    os.FileInfo
}

// Processor defines the interface for a file processing pipeline.
//
// Methods:
//   - Info: Returns a unique identifier or description of the processor instance.
//   - Process: Processes files received from a ScannerResult channel, uploads them to storage handlers,
//     and handles errors during the processing pipeline.
//
// Notes:
//   - Implementations of this interface are responsible for managing the processing of files,
//     including uploading to storage and handling post-upload actions.
//   - The `Process` method should ensure proper error handling and support context cancellation.
type Processor interface {
	Info() string
	Process(ctx context.Context, items <-chan ScannerResult, ech chan<- error)
}

// ProcessorResult represents the result of processing a file through the processor pipeline.
//
// Fields:
//   - Error: An error encountered during the processing of the file, if any.
//   - Path: The path of the file being processed.
//   - Result: A map where the key is the storage type (e.g., "S3", "Local") and the value is a pointer to the corresponding StorageResult.
//
// Notes:
//   - If the processing is successful, the Error field will be nil.
//   - The Result map contains the outcomes of the storage operations for the file across different storage handlers.
//   - This struct is used to communicate the overall outcome of the processing operation for a single file.
type ProcessorResult struct {
	Error   error
	Path    string
	TraceId string
	Result  map[string]*StorageResult
}

// Storage defines the interface for a storage handler that manages file storage operations.
//
// Methods:
//   - Info: Returns a unique identifier or description of the storage handler.
//   - Type: Returns the type of storage (e.g., "S3", "Local").
//   - Put: Handles the storage of a file, taking a ScannerResult as input and sending the result to a channel.
//
// Notes:
//   - Implementations of this interface are responsible for storing files and reporting the results of the operation.
//   - The `Put` method should handle errors gracefully and send a `StorageResult` to the provided channel.
type Storage interface {
	Info() string
	Type() string
	Put(ctx context.Context, item ScannerResult, candidates []string, stored chan<- StorageResult)
}

// StorageResult represents the result of a file storage operation.
//
// Fields:
//   - Error: An error encountered during the storage operation, if any.
//   - MarkerPath: The source directory of the file.
//   - Dest: The destination directory of the file.
//   - Type: The type of storage (e.g., "S3", "Local").
//   - Handler: The identifier of the uploader used for the storage operation.
//
// Notes:
//   - If the storage operation is successful, the Error field will be nil.
//   - This struct is used to communicate the outcome of a storage operation.
type StorageResult struct {
	Error         error
	MarkerPath    string
	UploadResults []*UploadInfo
	Type          string
	Handler       string
}

// UploadInfo represents metadata about a file upload operation.
//
// Fields:
//   - Src: The source directory of the file being uploaded.
//   - Dest: The destination directory where the file was uploaded.
//   - ChecksumType: The type of checksum used (e.g., "md5").
//   - Checksum: The checksum value of the uploaded file.
//   - Size: The size of the uploaded file in bytes.
//   - LastModified: The timestamp of the last modification of the uploaded file.
//
// Notes:
//   - This struct is used to provide detailed information about a file upload operation.
type UploadInfo struct {
	Src          string
	Dest         string
	ChecksumType string
	Checksum     string
	Size         int64
	LastModified time.Time
}
