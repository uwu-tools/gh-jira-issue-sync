package filesystem

import (
	"io/fs"
	"time"
)

type FakeFileInfo struct{}

func (f *FakeFileInfo) Name() string {
	return "filename.ext"
}

func (f *FakeFileInfo) Size() int64 {
	return 1
}

func (f *FakeFileInfo) Mode() fs.FileMode {
	return fs.ModeDir
}

func (f *FakeFileInfo) ModTime() time.Time {
	return time.Now()
}

func (f FakeFileInfo) IsDir() bool {
	return false
}

func (f FakeFileInfo) Sys() any {
	return "sys"
}
