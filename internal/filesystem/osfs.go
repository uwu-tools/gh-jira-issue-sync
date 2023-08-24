package filesystem

import "os"

type OsFs struct {
}

func (fs *OsFs) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (fs *OsFs) Getwd() (string, error) {
	return os.Getwd()
}
