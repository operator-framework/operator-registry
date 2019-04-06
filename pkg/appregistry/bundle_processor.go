package appregistry

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func NewBundleProcessor(logger *logrus.Entry, downloadPath string) (*bundleProcessor, error) {
	if downloadPath == "" {
		return nil, errors.New("folder to store downloaded operator bundle has not been specified")
	}

	return &bundleProcessor{
		logger: logger,
		base:   downloadPath,
	}, nil
}

type bundleProcessor struct {
	logger *logrus.Entry
	base   string
}

func (w *bundleProcessor) GetManifestDownloadDirectory() string {
	return w.base
}

// Process takes an item of the tar ball and writes it to the underlying file
// system.
func (w *bundleProcessor) Process(header *tar.Header, reader io.Reader) (done bool, err error) {
	const (
		directoryPerm = 0755
		fileFlag      = os.O_CREATE | os.O_RDWR
	)

	getType := func(header *tar.Header) string {
		switch header.Typeflag {
		case tar.TypeReg:
			return "file"
		case tar.TypeDir:
			return "directory"
		}

		return "unknown"
	}

	target := filepath.Join(w.base, header.Name)
	w.logger.Infof("%s - type=%s", target, getType(header))

	if header.Typeflag == tar.TypeDir {
		if _, err = os.Stat(target); err == nil {
			return
		}

		err = os.MkdirAll(target, directoryPerm)
		return
	}

	if header.Typeflag != tar.TypeReg {
		return
	}

	// It's a file.
	f, err := os.OpenFile(target, fileFlag, os.FileMode(header.Mode))
	if err != nil {
		return
	}

	defer f.Close()

	_, err = io.Copy(f, reader)
	return
}
