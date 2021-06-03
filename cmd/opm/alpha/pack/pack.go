package pack

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/karrick/godirwalk"
	"github.com/gabriel-vasile/mimetype"

	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"github.com/google/crfs/stargz"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type pack struct {
	configDir string

	debug          bool

	logger *logrus.Entry
}

func NewCmd() *cobra.Command {
	logger := logrus.New()
	p := pack{
		logger: logrus.NewEntry(logger),
	}
	cmd := &cobra.Command{
		Use:   "pack <source_path>",
		Short: "pack declarative configs into a seekable tar archive (stargz)",
		Long:  `pack declarative configs into a seekable tar archive (stargz). The tar headers store useful information
the contents of the index that clients can use to selectively unpack.`,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			p.configDir = args[0]
			if p.debug {
				logger.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return p.run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&p.debug, "debug", false, "enable debug logging")
	return cmd
}

func (p *pack) run(ctx context.Context) error {
	p.logger = p.logger.WithFields(logrus.Fields{"configs": p.configDir})

	type meta struct {
		Schema string `json:"schema"`
	}
	declcfgDetector := func(raw []byte, limit uint32) bool {
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 1024)
		for {
			doc := json.RawMessage{}
			if err := decoder.Decode(&doc); err != nil {
				return false
			}
			var in meta
			if err := json.Unmarshal(doc, &in); err == nil {
				return true
			}
		}
	}
	mimetype.Extend(declcfgDetector, "application/vnd.opm.declcfg.json", ".cfgjson")

	writer := stargz.NewWriter(os.Stdout)
	walkErr := godirwalk.Walk(p.configDir, &godirwalk.Options{
		Unsorted: true,
		Callback: func(path string, de *godirwalk.Dirent) error {
			dir, err := de.IsDirOrSymlinkToDir()
			if err != nil || dir == true{
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				logrus.Errorf("error opening %q: %v", path, err)
			}

			contents, err := io.ReadAll(f)
			if err != nil {
				logrus.Errorf("error reading %q: %v", path, err)
			}
			paxHeaders := map[string]string{}
			if mtype := mimetype.Detect(contents); mtype != nil {
				paxHeaders["OPM.mediatype"] = mtype.String()
			}
			var buf bytes.Buffer
			tarWriter := tar.NewWriter(&buf)
			if err := tarWriter.WriteHeader(&tar.Header{
				Name: path,
				Mode: 0600,
				Xattrs: paxHeaders,
				PAXRecords: paxHeaders,
				Size: int64(len(contents)),
			}); err != nil {
				logrus.Fatal(err)
			}
			if _, err := tarWriter.Write(contents); err != nil {
				logrus.Fatal(err)
			}
			if err := tarWriter.Close(); err != nil {
				logrus.Fatal(err)
			}
			if err := writer.AppendTar(bufio.NewReader(&buf)); err != nil {
				logrus.Fatal(err)
			}
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			logrus.Errorf("skipping %q: %v", osPathname, err)
			return godirwalk.SkipNode
		},
	})
	if walkErr != nil {
		return walkErr
	}
	return writer.Close()
}
