package certificates

import (
	"errors"
	"io/fs"
	"os"
)

// FileSystem models file operations.
type FileSystem interface {
	EnsureDirectory(path string, permissions fs.FileMode) error
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, permissions fs.FileMode) error
	Remove(path string) error
	FileExists(path string) (bool, error)
}

// OperatingSystemFileSystem interacts with the local filesystem.
type OperatingSystemFileSystem struct{}

// NewOperatingSystemFileSystem constructs an OperatingSystemFileSystem.
func NewOperatingSystemFileSystem() OperatingSystemFileSystem {
	return OperatingSystemFileSystem{}
}

// EnsureDirectory creates the directory hierarchy if necessary.
func (operatingSystemFileSystem OperatingSystemFileSystem) EnsureDirectory(path string, permissions fs.FileMode) error {
	err := os.MkdirAll(path, permissions)
	if err != nil {
		return err
	}
	return nil
}

// ReadFile returns the file contents.
func (operatingSystemFileSystem OperatingSystemFileSystem) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteFile persists the file contents.
func (operatingSystemFileSystem OperatingSystemFileSystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	err := os.WriteFile(path, data, permissions)
	if err != nil {
		return err
	}
	return nil
}

// Remove deletes the file if it exists.
func (operatingSystemFileSystem OperatingSystemFileSystem) Remove(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// FileExists reports whether the path exists.
func (operatingSystemFileSystem OperatingSystemFileSystem) FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
