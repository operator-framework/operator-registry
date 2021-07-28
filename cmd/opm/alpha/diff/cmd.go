package diff

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/operator-framework/operator-registry/internal/action"
	"github.com/operator-framework/operator-registry/internal/declcfg"
	containerd "github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/certs"
)

const (
	retryInterval = time.Second * 5
	timeout       = time.Minute * 1
)

type diff struct {
	oldRefs []string
	newRefs []string

	output string
	caFile string

	debug  bool
	logger *logrus.Entry
}

func NewCmd() *cobra.Command {
	a := diff{
		logger: logrus.NewEntry(logrus.New()),
	}
	cmd := &cobra.Command{
		Use:   "diff [old-refs]... new-refs...",
		Short: "Diff old and new catalog references into a declarative config",
		Long: templates.LongDesc(`
Diff a set of old and new catalog references ("refs") to produce a
declarative config containing only packages channels, and versions not present
in the old set. This is known as "latest" mode.
These references are passed through 'opm render' to produce a single declarative config.

This command has special behavior when old-refs are omitted, called "heads-only" mode:
instead of the output being that of 'opm render refs...'
(which would be the case given the preceding behavior description),
only the channel heads of all channels in all packages are included in the output,
and dependencies. Dependencies are assumed to be provided by either an old ref,
in which case they are not included in the diff, or a new ref, in which
case they are included. Dependencies provided by some catalog unknown to
'opm alpha diff' will not cause the command to error, but an error will occur
if that catalog is not serving these dependencies at runtime.
`),
		Example: templates.Examples(`
# Diff a catalog at some old state and latest state into a declarative config index.
mkdir -p catalog-index
opm alpha diff registry.org/my-catalog:abc123 registry.org/my-catalog:def456 -o yaml > ./my-catalog-index/index.yaml

# Build and push this index into an index image.
opm alpha generate dockerfile ./my-catalog-index
docker build -t registry.org/my-catalog:latest-abc123-def456 -f index.Dockerfile .
docker push registry.org/my-catalog:latest-abc123-def456

# Create a new catalog from the heads of an existing catalog, then build and push the image like above.
opm alpha diff registry.org/my-catalog:def456 -o yaml > my-catalog-index/index.yaml
docker build -t registry.org/my-catalog:headsonly-def456 -f index.Dockerfile .
docker push registry.org/my-catalog:headsonly-def456
`),
		Args: cobra.RangeArgs(1, 2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if a.debug {
				a.logger.Logger.SetLevel(logrus.DebugLevel)
			}
			a.logger.Logger.SetOutput(os.Stderr)
			return nil
		},
		RunE: a.addFunc,
	}

	cmd.Flags().StringVarP(&a.output, "output", "o", "yaml", "Output format (json|yaml)")
	cmd.Flags().StringVarP(&a.caFile, "ca-file", "", "", "the root Certificates to use with this command")

	cmd.Flags().BoolVar(&a.debug, "debug", false, "enable debug logging")
	return cmd
}

func (a *diff) addFunc(cmd *cobra.Command, args []string) error {
	a.parseArgs(args)

	skipTLS, err := cmd.Flags().GetBool("skip-tls")
	if err != nil {
		panic(err)
	}

	var write func(declcfg.DeclarativeConfig, io.Writer) error
	switch a.output {
	case "yaml":
		write = declcfg.WriteYAML
	case "json":
		write = declcfg.WriteJSON
	default:
		return fmt.Errorf("invalid --output value: %q", a.output)
	}

	rootCAs, err := certs.RootCAs(a.caFile)
	if err != nil {
		a.logger.Fatalf("error getting root CAs: %v", err)
	}
	reg, err := containerd.NewRegistry(containerd.SkipTLS(skipTLS), containerd.WithLog(a.logger), containerd.WithRootCAs(rootCAs))
	if err != nil {
		a.logger.Fatalf("error creating containerd registry: %v", err)
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			a.logger.Errorf("error destroying local cache: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	diff := action.Diff{
		Registry: reg,
		OldRefs:  a.oldRefs,
		NewRefs:  a.newRefs,
		Logger:   a.logger,
	}
	cfg, err := diff.Run(ctx)
	if err != nil {
		a.logger.Fatalf("error generating diff: %v", err)
	}

	if err := write(*cfg, os.Stdout); err != nil {
		a.logger.Fatalf("error writing diff: %v", err)
	}

	return nil
}

func (a *diff) parseArgs(args []string) {
	var old, new string
	switch len(args) {
	case 1:
		new = args[0]
	case 2:
		old, new = args[0], args[1]
	default:
		panic("should never be here, CLI must enforce arg size")
	}
	a.oldRefs = strings.Split(old, ",")
	a.newRefs = strings.Split(new, ",")
}
