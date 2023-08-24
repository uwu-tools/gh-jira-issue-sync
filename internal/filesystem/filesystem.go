package filesystem

import "os"

type Filesystem interface {
	Stat(name string) (os.FileInfo, error)
	Getwd() (string, error)
}
