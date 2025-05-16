package fsx

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// Walker is a utility for traversing file trees with a limit on the number of directory entries read at once.
// It manages opened files to optimize resource usage and ensures thread safety using a mutex.
type Walker struct {
	opened    map[string]*os.File // Map of currently opened files
	batchSize int                 // Maximum number of directory entries to read at once
	mu        sync.Mutex          // Mutex to protect access to the opened map
}

// NewWalker initializes and returns a new Walker instance.
//
// Parameters:
//   - batchSize: The maximum number of directory entries to read at once.
//
// Returns:
//   - A pointer to a new Walker instance.
//
// Notes:
//   - The batchSize parameter controls how many directory entries are read in a single operation.
func NewWalker(batchSize int) *Walker {
	return &Walker{
		opened:    make(map[string]*os.File),
		batchSize: batchSize,
	}
}

// Start traverses the file tree rooted at the specified directory, calling the provided WalkFunc for each file or directory.
//
// Parameters:
//   - root: The root directory to start the traversal.
//   - fn: A callback function to handle each file or directory encountered.
//
// Returns:
//   - An error if the traversal fails.
//
// Behavior:
//   - The traversal is performed in lexical order for deterministic results.
//   - Errors encountered during traversal are passed to the callback function.
//   - If the callback function returns filepath.SkipDir or filepath.SkipAll, the traversal is stopped accordingly.
//
// Notes:
//   - It is a copy of filepath.Walk, but uses a custom readDirEntries method to limit the number of entries read at once.
func (w *Walker) Start(root string, fn filepath.WalkFunc) error {
	info, err := os.Lstat(root)
	if err != nil {
		// if there is an error, ask the WalkFunc to handle it
		// and return the appropriate error
		err = fn(root, nil, err)
	} else {
		err = w.walk(root, info, fn)
	}

	if errors.Is(err, filepath.SkipDir) || errors.Is(err, filepath.SkipAll) {
		return nil
	}

	return err
}

// walk is a recursive helper function for traversing the file tree.
//
// Parameters:
//   - path: The current directory or file path being processed.
//   - info: FileInfo object for the current path.
//   - walkFn: A callback function to handle each file or directory encountered.
//
// Returns:
//   - An error if the traversal fails.
//
// Notes:
//   - This function uses readDirEntries to limit the number of directory entries read at once.
//   - It is similar to filepath.walk, but uses a custom readDirEntries method to limit the number of entries read at once.
func (w *Walker) walk(path string, info fs.FileInfo, walkFn filepath.WalkFunc) error {
	if !info.IsDir() {
		return walkFn(path, info, nil)
	}

	for {
		names, err := w.readDirEntries(path, w.batchSize)

		// Call the walkFn with the directory info including any error during reading the directory entries
		err1 := walkFn(path, info, err)

		// Handle errors from directory reading or callback function
		// If err is not nil, it indicates an error reading the directory so we should stop
		// If err1 is not nil, it indicates an error from the walkFn callback, so we should stop as well
		if err != nil || err1 != nil {
			// return whatever walkFn returns on error
			return err1
		}

		// No more entries to read, break the loop
		if len(names) == 0 {
			break
		}

		for _, name := range names {
			filename := filepath.Join(path, name)
			fileInfo, err := os.Lstat(filename)
			if err != nil {
				// If there is an error, ask the WalkFunc to handle it and return the appropriate error
				if err := walkFn(filename, fileInfo, err); err != nil && !errors.Is(err, filepath.SkipDir) {
					return err
				}
			} else {
				err = w.walk(filename, fileInfo, walkFn)
				if err != nil {
					if !fileInfo.IsDir() || !errors.Is(err, filepath.SkipDir) {
						return err
					}
				}
			}
		}
	}

	return nil
}

// readDirEntries reads a limited number of directory entries from the specified directory.
//
// Parameters:
//   - dirname: The directory to read entries from.
//   - n: The maximum number of entries to read.
//
// Returns:
//   - A slice of entry names.
//   - An error if reading the directory fails.
//
// Notes:
//   - The entries are returned in sorted order for deterministic traversal.
func (w *Walker) readDirEntries(dirname string, n int) ([]string, error) {
	f, err := w.getOrOpenFile(dirname)
	if err != nil {
		return nil, err
	}

	names, err := f.Readdirnames(n)
	if err != nil && !errors.Is(err, io.EOF) {
		return []string{}, w.closeFile(dirname, err)
	}

	if len(names) == 0 {
		return []string{}, w.closeFile(dirname, nil)
	}

	slices.Sort(names)
	return names, nil
}

// getOrOpenFile retrieves an open file from the map or opens it if not already opened.
//
// Parameters:
//   - name: The name of the file to retrieve or open.
//
// Returns:
//   - A pointer to the opened file.
//   - An error if opening the file fails.
//
// Notes:
//   - This function ensures thread safety using a mutex.
func (w *Walker) getOrOpenFile(name string) (*os.File, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if f, ok := w.opened[name]; ok {
		return f, nil
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	w.opened[name] = f
	return f, nil
}

// closeFile closes a file and removes it from the opened map.
//
// Parameters:
//   - name: The name of the file to close.
//   - prevError: An error to return if closing the file fails.
//
// Returns:
//   - An error if closing the file fails, combined with the previous error if any.
//
// Notes:
//   - This function ensures thread safety using a mutex.
func (w *Walker) closeFile(name string, prevError error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, ok := w.opened[name]
	if !ok {
		return prevError // Nothing to close, return the previous error
	}

	if err := f.Close(); err != nil {
		return errors.Join(err, prevError)
	}

	delete(w.opened, name)
	return prevError
}

// End releases all resources held by the Walker.
//
// Behavior:
//   - Closes all open files and clears the opened map.
//
// Notes:
//   - This method ensures that all resources are properly released when the Walker is no longer needed.
func (w *Walker) End() {
	if w == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, f := range w.opened {
		_ = f.Close() // Ignore errors during cleanup
	}

	w.opened = make(map[string]*os.File)
}
