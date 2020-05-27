package dns

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

func TestEnsureNsswitch(t *testing.T) {
	tests := []struct {
		name         string
		goos         string
		existingFile bool
		wantFile     bool
		wantErr      bool
	}{
		{
			name:         "no file",
			goos:         "linux",
			existingFile: false,
			wantFile:     true,
			wantErr:      false,
		},
		{
			name:         "existing file",
			goos:         "linux",
			existingFile: true,
			wantFile:     false,
			wantErr:      false,
		},
		{
			name:     "windows",
			goos:     "windows",
			wantFile: false,
			wantErr:  false,
		},
		{
			name:     "mac",
			goos:     "darwin",
			wantFile: false,
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GOOS = tt.goos
			// don't want to overwrite the real nsswitch
			NsswitchFilename = "testfile"

			if tt.existingFile {
				require.NoError(t, ioutil.WriteFile(NsswitchFilename, []byte("test"), 0644))
			}

			if err := EnsureNsswitch(); (err != nil) != tt.wantErr {
				t.Errorf("EnsureNsswitch() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantFile {
				contents, err := ioutil.ReadFile(NsswitchFilename)
				require.NoError(t, err)
				require.Equal(t, NsswitchContents, contents)
				os.Remove(NsswitchFilename)
			}
			if tt.existingFile {
				contents, err := ioutil.ReadFile(NsswitchFilename)
				require.NoError(t, err)
				require.NotEqual(t, NsswitchContents, contents)
				os.Remove(NsswitchFilename)
			}
		})
	}
}
