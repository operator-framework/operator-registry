package containertools

import (
	"archive/tar"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
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
// 2. setup untarer
// 4. read tar archive contents to a temporary directory on disk
// 5. Ensure file contents are there
// 6. Ensure permissions are set as expected
func TestUntarer(t *testing.T) {
	log := logrus.NewEntry(logrus.StandardLogger())
	temp, err := ioutil.TempDir("./", "temp-")
	if err != nil {
		t.Error(err)
	}

	defer os.RemoveAll(temp)

	piper, pipew := io.Pipe()
	defer func() {
		pipew.Close()
		piper.Close()
	}()

	var (
		wg          sync.WaitGroup
		ctx, cancel = context.WithCancel(context.Background())
		untarer     = newUntarer(log)
	)
	defer cancel()

	tr := tar.NewReader(piper)
	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := untarer.Untar(ctx, tr, temp); err != nil {
			t.Error(err)
		}
	}()

	tw := tar.NewWriter(pipew)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: int64(0600),
			Size: int64(len([]byte(content))),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Error(err)
		}
		if _, err = tw.Write([]byte(content)); err != nil {
			t.Error(err)
		}
	}

	tw.Close()
	wg.Wait()

	dir, err := os.ReadDir(temp)
	if err != nil {
		t.Error(err)
	}

	var found bool
	for f := range files {
		found = false
		for _, entry := range dir {
			if f == entry.Name() {
				found = true
				break
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

		// The file mode is changed from 0777 to 0755 due to the default umask 022
		if info.Mode() != os.FileMode(0755) {
			t.Errorf("unexpected file mode %s, expected %s", info.Mode(), os.FileMode(0755))
		}

		// Check contents match expected
		key := entry.Name()
		content, err := ioutil.ReadFile(filepath.Join(temp, key))
		if !reflect.DeepEqual(content, []byte(files[key])) {
			t.Errorf("file %s does not match after extraction: got %s expected %s", key, content, files[key])
		}
	}
}

func TestUntarDirectory(t *testing.T) {
	log := logrus.NewEntry(logrus.StandardLogger())
	temp, err := ioutil.TempDir("./", "temp-")
	if err != nil {
		t.Error(err)
	}

	defer os.RemoveAll(temp)

	piper, pipew := io.Pipe()
	defer func() {
		pipew.Close()
		piper.Close()
	}()

	var (
		wg          sync.WaitGroup
		ctx, cancel = context.WithCancel(context.Background())
		untarer     = newUntarer(log)
	)
	defer cancel()

	tr := tar.NewReader(piper)
	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := untarer.Untar(ctx, tr, temp); err != nil {
			t.Error(err)
		}
	}()

	// tree tar has the following format
	// tree
	//	.
	//	├── a
	//	│   └── a.txt
	//	└── b
	//		└── b.txt
	tree, err := os.Open("./testdata/tree.tar")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(pipew, tree)

	// check temp for results
	a, err := os.Stat(filepath.Join(temp + "/tree/a"))
	if err != nil {
		t.Fatal(err)
	}
	if !a.IsDir() {
		t.Fatal("expected /a dir at the top level")
	}

	b, err := os.Stat(filepath.Join(temp + "/tree/b"))
	if err != nil {
		t.Fatal(err)
	}
	if !b.IsDir() {
		t.Fatal("expected /b dir at the top level")
	}
}
