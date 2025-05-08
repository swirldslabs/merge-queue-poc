package fsx

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func PathExists(filePath string) (os.FileInfo, bool) {
	s, err := os.Stat(filePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return s, false
	}

	return s, true
}

func Copy(src string, dst string, perm os.FileMode) error {
	// Open the source file
	inputFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("couldn't open source file: %w", err)
	}
	defer CloseFile(inputFile)

	// Open the destination file
	outputFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("couldn't open destination file: %w", err)
	}
	defer CloseFile(outputFile)

	// Copy the contents from the source to the destination
	if _, err = io.Copy(outputFile, inputFile); err != nil {
		return fmt.Errorf("couldn't copy to destination from source: %w", err)
	}

	// Flush the output file to ensure all data is written
	if err = outputFile.Sync(); err != nil {
		return fmt.Errorf("failed to flush destination file: %w", err)
	}

	CloseFile(outputFile)

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

func SplitFilePath(filePath string) (dir, fileNameWithoutExt, ext string) {
	dir, file := path.Split(filePath)
	ext = path.Ext(file)
	fileNameWithoutExt = strings.TrimSuffix(file, ext)
	return dir, fileNameWithoutExt, ext
}

func CombineFilePath(dir string, fileName string, ext string) string {
	return path.Join(dir, fmt.Sprintf("%s%s", fileName, ext))
}

func CloseFile(file *os.File) {
	if file == nil {
		return
	}

	if err := file.Close(); err != nil {
		fmt.Printf("warning: failed to close file: %v\n", err)
	}
}

func RemoveFile(file string) {
	if err := os.Remove(file); err != nil {
		fmt.Printf("warning: failed to remove file: %v\n", err)
	}
}

func FileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}

	defer CloseFile(file)

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("failed to compute hash of the file: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func FileSha256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	defer CloseFile(file)

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
