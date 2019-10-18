package rest

import (
	"io"
	"os"
	"io/ioutil"
	"go-search/conf"
)

func saveTmpFile(in io.ReadCloser) (string, io.ReadCloser, error) {
	tmpfile, err := ioutil.TempFile(conf.ServiceConf.RootDir, "tmp")
	if err != nil {
		return "", nil, err
	}
	name := tmpfile.Name()

	if _, err = io.Copy(tmpfile, in); err != nil {
		defer os.Remove(name)
		tmpfile.Close()
		return "", nil, err
	}
	tmpfile.Close()
	fp, err := os.Open(name)
	if err != nil {
		defer os.Remove(name)
		return "", nil, err
	}
	return name, fp, nil
}
