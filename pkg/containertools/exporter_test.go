package containertools

import (
	"archive/tar"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

var files = map[string]string{
	"index.html": `<body>Hello!</body>`,
	"lang.json":  `[{"code":"eng","name":"English"}]`,
	"songs.txt":  `Claire de la lune, The Valkyrie, Swan Lake`,
}

// 1. create a tar archive in memory
// 2. setup Exporter with Writer pipe
// 4. read tar archive contents to a temporary directory on disk
// 5. Ensure file contents are there
// 6. Ensure permissions are set as expected
func TestExporter_Run(t *testing.T) {
	log := logrus.NewEntry(&logrus.Logger{})
	temp, err := ioutil.TempDir("./", "temp-")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(temp)

	exporter, err := NewExporter(temp, log)
	if err != nil || exporter == nil {
		t.Error(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := exporter.Run()
		if err != nil {
			t.Error(err)
		}
	}(&wg)

	tw := tar.NewWriter(exporter.Writer())
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: int64(0600),
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Error(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Error(err)
		}
	}

	tw.Close()
	wg.Wait()

	dir, err := os.ReadDir(temp)
	if err != nil {
		t.Error(err)
	}

	// check all files are on disk
	var found bool
	for f := range files {
		found = false
		for _, entry := range dir {
			if f == entry.Name() {
				found = true
				continue
			}
		}
	}

	if !found {
		t.Error("did not find expected file in tar archive on disk")
	}

	for _, entry := range dir {
		info, err := entry.Info()
		if err != nil {
			t.Error(err)
		}
		// the file mode is changed from 0777 to 0755 due to the default umask 022
		if info.Mode() != os.FileMode(0755) {
			t.Errorf("unexpected file mode %s, expected %s", info.Mode(), os.FileMode(0755))
		}
	}
}
