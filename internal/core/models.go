package core

import (
	"context"
	"os"
)

// Scanner defines the interface for a file scanning component.
//
// Methods:
//   - Info: Returns a unique identifier or description of the scanner instance.
//   - Scan: Scans a given path (file or directory) and streams the results through a channel.
//     Errors encountered during the scanning process are sent to an error channel.
//
// Notes:
//   - Implementations of this interface are responsible for traversing directories or files
//     and providing metadata about the scanned items.
//   - The `Scan` method should support context cancellation to allow graceful termination of the scanning process.
type Scanner interface {
	Info() string
	Scan(ctx context.Context, path string, ech chan<- error) <-chan ScannerResult
}

// ScannerResult represents the result of scanning a file or directory.
//
// Fields:
//   - Path: The path of the file or directory that was scanned.
//   - Info: The file information (os.FileInfo) associated with the scanned file or directory.
//
// Notes:
//   - This struct is used to communicate the details of a scanned file or directory.
//   - The `Path` field provides the location of the scanned item, while the `Info` field contains metadata such as size, modification time, etc.
type ScannerResult struct {
	Path string
	Info os.FileInfo
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
	Error  error
	Path   string
	Result map[string]*StorageResult
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
	Put(ctx context.Context, item ScannerResult, stored chan<- StorageResult)
}

// StorageResult represents the result of a file storage operation.
//
// Fields:
//   - Error: An error encountered during the storage operation, if any.
//   - Src: The source path of the file being stored.
//   - Dest: The destination path where the file was stored.
//   - Type: The type of storage (e.g., "S3", "Local").
//   - Uploader: The identifier of the uploader used for the storage operation.
//
// Notes:
//   - If the storage operation is successful, the Error field will be nil.
//   - This struct is used to communicate the outcome of a storage operation.
type StorageResult struct {
	Error    error
	Src      string
	Dest     string
	Type     string
	Uploader string
}
