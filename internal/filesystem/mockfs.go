package filesystem

import (
	"github.com/stretchr/testify/mock"
	"os"
)

type MockFs struct {
	mock.Mock
}

func (fs *MockFs) Stat(name string) (os.FileInfo, error) {
	args := fs.Called(name)
	return args.Get(0).(os.FileInfo), args.Error(1) //nolint:wrapcheck
}

func (fs *MockFs) Getwd() (string, error) {
	args := fs.Called()
	return args.String(0), args.Error(1) //nolint:wrapcheck
}
