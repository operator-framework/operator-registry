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

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
	containerd "github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/certs"
)

const (
	retryInterval = time.Second * 5
	timeout       = time.Minute * 1
)

type input struct {
	oldRefs         []string
	newRefs         []string
	skipDeps        bool
	includeAdditive bool
	headsOnly       bool
	includeFile     string

	output string
	caFile string

	debug  bool
	logger *logrus.Entry
}

// Example include file needs to be formatted separately so indentation is not messed up.
var includeFileExample = fmt.Sprintf(`packages:
%[1]s- name: foo
%[1]s- name: bar
%[1]s  channels:
%[1]s  - name: stable
%[1]s- name: baz
%[1]s  channels:
%[1]s  - name: alpha
%[1]s    versions:
%[1]s    - 0.2.0-alpha.0`, templates.Indentation)

func NewCmd() *cobra.Command {
	in := input{
		logger: logrus.NewEntry(logrus.New()),
	}
	cmd := &cobra.Command{
		Use:   "diff [old-refs]... new-refs...",
		Short: "Diff old and new catalog references into a declarative config",
		Long: templates.LongDesc(`
'diff' returns a declarative config containing packages, channels, and versions
from new-refs, optionally removing those in old-refs or those omitted by an include config file.

Each set of refs is passed to 'opm render <refs>' to produce a single, normalized delcarative config.

Depending on what arguments are provided to the command, a particular "mode" is invoked to produce a diff:

- If in heads-only mode (old-refs is not specified), then the heads of channels in new-refs are added to the output.
- If in latest mode (old-refs is specified), a diff between old-refs and new-refs is added to the output.
- If --include-file is set, items from that file will be added to the diff:
	- If --include-additive is false (the default), a diff will be generated only on those objects, depending on the mode.
	- If --include-additive is true, the diff will contain included objects, plus those added by the mode's invocation.

Dependencies are added in all modes if --skip-deps is false (the default).
Dependencies are assumed to be provided by either an old-ref, in which case they are not included in the diff,
or a new-ref, in which case they are included.
Dependencies provided by some catalog unknown to 'diff' will not cause the command to error,
but an error will occur if that catalog is not serving these dependencies at runtime.
While dependency inclusion can be turned off with --skip-deps, doing so is not recommended
unless you are certain some in-cluster catalog satisfies all dependencies.
`),
		Example: fmt.Sprintf(templates.Examples(`
# Create a directory for your declarative config diff.
mkdir -p my-catalog-index

# THEN:
# Create a new catalog from a diff between an old and the latest
# state of a catalog as a declarative config index.
opm alpha diff registry.org/my-catalog:abc123 registry.org/my-catalog:def456 -o yaml > ./my-catalog-index/index.yaml

# OR:
# Create a new catalog from the heads of an existing catalog.
opm alpha diff registry.org/my-catalog:def456 -o yaml > my-catalog-index/index.yaml

# OR:
# Only include all of package "foo", package "bar" channel "stable",
# and package "baz" channel "alpha" version "0.2.0-alpha.0" (and its upgrade graph) in the diff.
cat <<EOF > include.yaml
%s
EOF
opm alpha diff registry.org/my-catalog:def456 -i include.yaml -o yaml > pruned-index/index.yaml

# OR:
# Include all of package "foo", package "bar" channel "stable",
# and package "baz" channel "alpha" version "0.2.0-alpha.0" in the diff
# on top of heads of all other channels in all packages (using the above include.yaml).
opm alpha diff registry.org/my-catalog:def456 -i include.yaml --include-additive -o yaml > pruned-index/index.yaml

# FINALLY:
# Build an index image containing the diff-ed declarative config,
# then tag and push it.
opm alpha generate dockerfile ./my-catalog-index
docker build -t registry.org/my-catalog:diff-latest -f index.Dockerfile .
docker push registry.org/my-catalog:diff-latest
`), includeFileExample),
		Args: cobra.RangeArgs(1, 2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if in.debug {
				in.logger.Logger.SetLevel(logrus.DebugLevel)
			}
			in.logger.Logger.SetOutput(os.Stderr)
			return nil
		},
		RunE: in.diffFunc,
	}

	cmd.Flags().BoolVar(&in.skipDeps, "skip-deps", false, "do not include bundle dependencies in the output catalog")

	cmd.Flags().StringVarP(&in.output, "output", "o", "yaml", "Output format (json|yaml)")
	cmd.Flags().StringVar(&in.caFile, "ca-file", "", "the root Certificates to use with this command")
	cmd.Flags().StringVarP(&in.includeFile, "include-file", "i", "",
		"YAML defining packages, channels, and/or bundles/versions to extract from the new refs. "+
			"Upgrade graphs from individual bundles/versions to their channel's head are also included")
	cmd.Flags().BoolVar(&in.includeAdditive, "include-additive", false,
		"Ref objects from --include-file are returned on top of 'heads-only' or 'latest' output")
	cmd.Flags().BoolVar(&in.headsOnly, "headsOnly", false,
		"Using `headsOnly` mode where head(s) of the channel(s) in the package are selected")

	cmd.Flags().BoolVar(&in.debug, "debug", false, "enable debug logging")
	return cmd
}

func (in *input) diffFunc(cmd *cobra.Command, args []string) error {
	in.parseArgs(args)

	if cmd.Flags().Changed("include-additive") && in.includeFile == "" {
		in.logger.Fatal("must set --include-file if --include-additive is set")
	}

	var write func(declcfg.DeclarativeConfig, io.Writer) error
	switch in.output {
	case "yaml":
		write = declcfg.WriteYAML
	case "json":
		write = declcfg.WriteJSON
	default:
		return fmt.Errorf("invalid --output value: %q", in.output)
	}

	skipTLSVerify, useHTTP, err := util.GetTLSOptions(cmd)
	if err != nil {
		return err
	}

	rootCAs, err := certs.RootCAs(in.caFile)
	if err != nil {
		in.logger.Fatalf("error getting root CAs: %v", err)
	}
	reg, err := containerd.NewRegistry(
		containerd.SkipTLSVerify(skipTLSVerify),
		containerd.WithLog(in.logger),
		containerd.WithRootCAs(rootCAs),
		containerd.WithPlainHTTP(useHTTP),
	)
	if err != nil {
		in.logger.Fatalf("error creating containerd registry: %v", err)
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			in.logger.Errorf("error destroying local cache: %v", err)
		}
	}()

	diff := action.Diff{
		Registry:          reg,
		OldRefs:           in.oldRefs,
		NewRefs:           in.newRefs,
		SkipDependencies:  in.skipDeps,
		IncludeAdditively: in.includeAdditive,
		HeadsOnly:         in.headsOnly,
		Logger:            in.logger,
	}

	if in.includeFile != "" {
		f, err := os.Open(in.includeFile)
		if err != nil {
			in.logger.Fatalf("error opening include file: %v", err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				in.logger.Error(cerr)
			}
		}()
		if diff.IncludeConfig, err = action.LoadDiffIncludeConfig(f); err != nil {
			in.logger.Fatalf("error loading include file: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	cfg, err := diff.Run(ctx)
	if err != nil {
		in.logger.Fatalf("error generating diff: %v", err)
	}

	if err := write(*cfg, os.Stdout); err != nil {
		in.logger.Fatalf("error writing diff: %v", err)
	}

	return nil
}

func (in *input) parseArgs(args []string) {
	var old, new string
	switch len(args) {
	case 1:
		new = args[0]
	case 2:
		old, new = args[0], args[1]
	default:
		logrus.Panic("should never be here, CLI must enforce arg size")
	}
	if old != "" {
		in.oldRefs = strings.Split(old, ",")
	}
	in.newRefs = strings.Split(new, ",")
}
