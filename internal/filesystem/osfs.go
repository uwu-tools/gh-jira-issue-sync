package filesystem

import (
	"fmt"
	"os"
)

type OsFs struct{}

func (fs *OsFs) Stat(name string) (os.FileInfo, error) {
	fileInfo, err := os.Stat(name)
	return fileInfo, fmt.Errorf("os.Stat: %w", err)
}

func (fs *OsFs) Getwd() (string, error) {
	currentWd, err := os.Getwd()
	return currentWd, fmt.Errorf("os.Getwd: %w", err)
}
