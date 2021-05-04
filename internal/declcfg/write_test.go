package declcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestWriteDir(t *testing.T) {
	type spec struct {
		name      string
		cfg       DeclarativeConfig
		setupDir  func() (string, error)
		assertion require.ErrorAssertionFunc
	}
	setupNonExistentDir := func() (string, error) {
		return filepath.Join(os.TempDir(), "decl-write-dir-"+rand.String(5)), nil
	}
	setupEmptyDir := func() (string, error) { return ioutil.TempDir("", "decl-write-dir-") }
	setupNonEmptyDir := func() (string, error) {
		dir, err := ioutil.TempDir("", "decl-write-dir-")
		if err != nil {
			return "", err
		}
		if _, err := ioutil.TempFile(dir, "decl-write-dir-file-"); err != nil {
			return "", err
		}
		return dir, nil
	}
	setupFile := func() (string, error) {
		f, err := ioutil.TempFile("", "decl-write-dir-file-")
		if err != nil {
			return "", err
		}
		return f.Name(), nil
	}

	specs := []spec{
		{
			name:      "Success/NonExistentDir",
			cfg:       buildValidDeclarativeConfig(true),
			setupDir:  setupNonExistentDir,
			assertion: require.NoError,
		},
		{
			name:      "Success/EmptyDir",
			cfg:       buildValidDeclarativeConfig(true),
			setupDir:  setupEmptyDir,
			assertion: require.NoError,
		},
		{
			name:      "Error/NotADir",
			cfg:       buildValidDeclarativeConfig(true),
			setupDir:  setupFile,
			assertion: require.Error,
		},
		{
			name:      "Error/NonEmptyDir",
			cfg:       buildValidDeclarativeConfig(true),
			setupDir:  setupNonEmptyDir,
			assertion: require.Error,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			testDir, err := s.setupDir()
			require.NoError(t, err)
			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			err = WriteDir(s.cfg, testDir)
			s.assertion(t, err)
			if err == nil {
				entries, err := ioutil.ReadDir(testDir)
				require.NoError(t, err)
				entryNames := []string{}
				for _, f := range entries {
					entryNames = append(entryNames, f.Name())
				}

				expectedEntryNames := []string{
					fmt.Sprintf("%s.json", globalName),
					"anakin",
					"boba-fett",
				}
				require.ElementsMatch(t, expectedEntryNames, entryNames)

				anakinFilename := filepath.Join(testDir, "anakin", "anakin.json")
				anakinFile, err := os.Open(anakinFilename)
				require.NoError(t, err)
				defer anakinFile.Close()
				anakin, err := readYAMLOrJSON(anakinFile)
				require.NoError(t, err)
				assert.Len(t, anakin.Packages, 1)
				assert.Len(t, anakin.Bundles, 3)
				assert.Len(t, anakin.Others, 1)

				bobaFettFilename := filepath.Join(testDir, "boba-fett", "boba-fett.json")
				bobaFettFile, err := os.Open(bobaFettFilename)
				require.NoError(t, err)
				defer bobaFettFile.Close()
				bobaFett, err := readYAMLOrJSON(bobaFettFile)
				require.NoError(t, err)
				assert.Len(t, bobaFett.Packages, 1)
				assert.Len(t, bobaFett.Bundles, 2)
				assert.Len(t, bobaFett.Others, 1)

				globalFilename := filepath.Join(testDir, fmt.Sprintf("%s.json", globalName))
				globalFile, err := os.Open(globalFilename)
				require.NoError(t, err)
				globals, err := readYAMLOrJSON(globalFile)
				require.NoError(t, err)
				assert.Len(t, globals.Packages, 0)
				assert.Len(t, globals.Bundles, 0)
				assert.Len(t, globals.Others, 2)

				all, err := LoadDir(testDir)
				require.NoError(t, err)

				assert.Len(t, all.Packages, 2)
				assert.Len(t, all.Bundles, 5)
				assert.Len(t, all.Others, 4)
			}
		})
	}
}

func TestWriteLoadRoundtrip(t *testing.T) {
	type spec struct {
		name  string
		write func(DeclarativeConfig, string) error
		load  func(string) (*DeclarativeConfig, error)
	}

	specs := []spec{
		{
			name:  "Dir",
			write: WriteDir,
			load:  LoadDir,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			to := buildValidDeclarativeConfig(true)

			filename := filepath.Join(os.TempDir(), "declcfg-"+rand.String(5))
			defer func() {
				require.NoError(t, os.RemoveAll(filename))
			}()
			require.NoError(t, s.write(to, filename))

			from, err := s.load(filename)
			require.NoError(t, err)

			equalsDeclarativeConfig(t, to, *from)
		})
	}
}

func removeJSONWhitespace(cfg *DeclarativeConfig) {
	for ib := range cfg.Bundles {
		for ip := range cfg.Bundles[ib].Properties {
			var buf bytes.Buffer
			json.Compact(&buf, cfg.Bundles[ib].Properties[ip].Value)
			cfg.Bundles[ib].Properties[ip].Value = buf.Bytes()
		}
	}
	for io := range cfg.Others {
		var buf bytes.Buffer
		json.Compact(&buf, cfg.Others[io].Blob)
		cfg.Others[io].Blob = buf.Bytes()
	}
}
