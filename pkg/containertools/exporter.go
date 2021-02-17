package containertools

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Exporter can read a tar archive being passed along from a writer and write it to a local destination.
type Exporter struct {
	dest   string
	writer io.WriteCloser
	reader *tar.Reader
	log    *logrus.Entry
}

func NewExporter(dest string, logger *logrus.Entry) (*Exporter, error) {
	// use stdout as pipew
	piper, pipew := io.Pipe()
	d, err := filepath.Abs(dest)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		dest:   d,
		writer: pipew,
		reader: tar.NewReader(piper),
		log:    logger,
	}, nil
}

func (e *Exporter) Writer() io.WriteCloser {
	return e.writer
}

func (e *Exporter) Close() error {
	return e.writer.Close()
}

// Run reads the tar stream from reader as it comes from stdout and writes the file/dir to the destination specified.
func (e *Exporter) Run() error {
	for {
		header, err := e.reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// determine proper file path info
		finfo := header.FileInfo()
		fileName := header.Name
		absFileName := filepath.Join(e.dest, fileName)

		// if a dir, create it, then go to next segment
		if finfo.Mode().IsDir() {
			if err := os.MkdirAll(absFileName, os.ModePerm); err != nil {
				e.log.Debugf("creating %s dir", absFileName)
				return err
			}
			continue
		}

		// create new file with custom file permissions
		// TODO sync with existing containerd permissions
		file, err := os.OpenFile(
			absFileName,
			os.O_RDWR|os.O_CREATE|os.O_TRUNC,
			os.ModePerm,
		)
		if err != nil {
			return err
		}

		e.log.Debugf("writing %s to disk", absFileName)
		n, err := io.Copy(file, e.reader)
		if err != nil {
			return err
		}
		if closeErr := file.Close(); closeErr != nil {
			return closeErr
		}
		if n != finfo.Size() {
			return fmt.Errorf("unpacking to disk: wrote %d, want %d", n, finfo.Size())
		}
	}

	e.log.Debugf("extracted contents from container filesystem")
	return nil
}
