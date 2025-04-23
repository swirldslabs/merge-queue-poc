package fsx

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func PathExists(filePath string) bool {
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func Copy(src string, dst string, perm os.FileMode) error {
	// Open the source file
	inputFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("couldn't open source file: %w", err)
	}
	defer func() {
		if cerr := inputFile.Close(); cerr != nil {
			fmt.Printf("warning: failed to close source file: %v\n", cerr)
		}
	}()

	// Open the destination file
	outputFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("couldn't open destination file: %w", err)
	}
	defer func() {
		if cerr := outputFile.Close(); cerr != nil {
			fmt.Printf("warning: failed to close destination file: %v\n", cerr)
		}
	}()

	// Copy the contents from the source to the destination
	if _, err = io.Copy(outputFile, inputFile); err != nil {
		return fmt.Errorf("couldn't copy to destination from source: %w", err)
	}

	// Flush the output file to ensure all data is written
	if err = outputFile.Sync(); err != nil {
		return fmt.Errorf("failed to flush destination file: %w", err)
	}

	return nil
}

func Move(src string, dst string, perm os.FileMode) error {
	err := Copy(src, dst, perm)
	if err != nil {
		return err
	}

	err = os.Remove(src)
	if err != nil {
		return fmt.Errorf("couldn't remove source file: %v", err)
	}

	return nil
}
