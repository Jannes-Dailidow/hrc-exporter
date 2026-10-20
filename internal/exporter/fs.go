package exporter

import (
	"io/fs"
	"os"
)

// OSFileSystem is the FileSystem backed by the local disk.
type OSFileSystem struct{}

func (OSFileSystem) MkdirAll(path string) error { return os.MkdirAll(path, 0o755) }

func (OSFileSystem) CreateTemp(dir, pattern string) (TempFile, error) {
	return os.CreateTemp(dir, pattern)
}

func (OSFileSystem) Rename(oldPath, newPath string) error  { return os.Rename(oldPath, newPath) }
func (OSFileSystem) Remove(path string) error              { return os.Remove(path) }
func (OSFileSystem) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }
