package unpack

import (
	"context"
	"fmt"
	"io"

	"github.com/google/crfs/stargz"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

type unpack struct {
	archivePath string

	debug          bool

	logger *logrus.Entry
}

func NewCmd() *cobra.Command {
	logger := logrus.New()
	u := unpack{
		logger: logrus.NewEntry(logger),
	}
	cmd := &cobra.Command{
		Use:   "unpack <stargz_path>",
		Short: "unpack declarative configs from a seekable tar archive (stargz)",
		Long:  `unpack declarative configs from a seekable tar archive (stargz). The tar headers store useful information
the contents of the index that clients can use to selectively unpack.`,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			u.archivePath = args[0]
			if u.debug {
				logger.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return u.run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&u.debug, "debug", false, "enable debug logging")
	return cmd
}

func (u *unpack) run(ctx context.Context) error {
	u.logger = u.logger.WithFields(logrus.Fields{"configs": u.archivePath})

	f, err := os.Open(u.archivePath)
	if err != nil {
		logrus.Fatalf("error opening %q: %v", u.archivePath, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		logrus.Fatalf("stat error %q: %v", u.archivePath, err)
	}
	r, err := stargz.Open(io.NewSectionReader(f, 0, fi.Size()))
	if err != nil {
		logrus.Fatalf("unpack error %q: %v", u.archivePath, err)
	}
	root, ok := r.Lookup("")
	if !ok {
		logrus.Fatalf("unpack error %q: %v", u.archivePath, err)
	}
	root.ForeachChild(walk)
	return nil
}

func walk(baseName string, ent *stargz.TOCEntry) bool {
	if ent.Stat().IsDir() {
		ent.ForeachChild(walk)
		return true
	}
	if len(ent.Xattrs) == 0 {
		fmt.Println(ent.Name)
		ent.ForeachChild(walk)
		return true
	}
	mtype, ok := ent.Xattrs["OPM.mediatype"]
	if !ok {
		fmt.Println(ent.Name)
		ent.ForeachChild(walk)
		return true
	}
	fmt.Println(ent.Name, string(mtype))
	ent.ForeachChild(walk)
	return true
}