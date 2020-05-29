package loader

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

// LoadLocalFile loads a file from the local filesystem.
func LoadLocalFile(path string) ([]byte, error) {
	if fi, err := os.Stat(path); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("cannot load a directory")
	}

	return ioutil.ReadFile(path)
}
