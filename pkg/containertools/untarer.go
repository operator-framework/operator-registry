package containertools

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// untarer can untar tar files.
type untarer struct {
	log *logrus.Entry
}

func newUntarer(logger *logrus.Entry) *untarer {
	return &untarer{
		log: logger,
	}
}

func (u *untarer) Untar(ctx context.Context, reader *tar.Reader, path string) error {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	u.log.Debugf("untarer writing to %s", path)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := u.expandHeader(reader, header, path); err != nil {
			return err
		}

	}

	u.log.Debugf("untarer extracted contents from container filesystem")
	return nil
}

func (u *untarer) expandHeader(reader *tar.Reader, header *tar.Header, base string) error {
	// Determine proper file path info
	info := header.FileInfo()
	name := header.Name
	path := filepath.Join(base, name)

	// If a dir, create it, then go to next segment
	if info.Mode().IsDir() {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			u.log.Debugf("creating %s dir", path)
			return err
		}

		return nil
	}

	// Create new file with custom file permissions
	file, err := os.OpenFile(
		path,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC,
		os.ModePerm,
	)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			u.log.Warnf("error closing file at %s: %s", path, err)
		}
	}()

	u.log.Debugf("untarer writing %s to disk", path)
	n, err := io.Copy(file, reader)
	if err != nil {
		return err
	}

	if n != info.Size() {
		return fmt.Errorf("unpacking to disk: wrote %d, want %d", n, info.Size())
	}

	return nil
}
