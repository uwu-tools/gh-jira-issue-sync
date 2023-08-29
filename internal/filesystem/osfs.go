package filesystem

import (
	"os"
)

type OsFs struct{}

func (fs *OsFs) Stat(name string) (os.FileInfo, error) {
	fileInfo, err := os.Stat(name)
	return fileInfo, err //nolint:wrapcheck
}

func (fs *OsFs) Getwd() (string, error) {
	currentWd, err := os.Getwd()
	return currentWd, err //nolint:wrapcheck
}
