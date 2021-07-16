package inflate

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	gojson "cuelang.org/go/encoding/json"
	goyaml "cuelang.org/go/encoding/yaml"
	"cuelang.org/go/pkg/encoding/json"
	"cuelang.org/go/pkg/encoding/yaml"
	"github.com/spf13/cobra"
)

//go:embed builtins/simple.cue
var simple string

//go:embed builtins/semver.cue
var semver string

var strategies = map[string]string{
	"simple": simple,
	"semver": semver,
}

type MarshallFunc func(value cue.Value) (string, error)

func NewCmd() *cobra.Command {
	var (
		output string
		strategy string
	)
	cmd := &cobra.Command{
		Use:   "inflate [input]",
		Short: "Generate declarative config blobs from a simplified veneer.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var marshaller MarshallFunc
			switch output {
			case "yaml":
				marshaller = yaml.MarshalStream
			case "json":
				marshaller = json.MarshalStream
			default:
				log.Fatalf("invalid --output value %q, expected (json|yaml)", output)
			}
			strat, ok := strategies[strategy]
			if !ok {
				// assume a file if we can't find a builtin strategy
				fileStrategy, err := os.ReadFile(strategy)
				if err != nil {
					log.Fatalf("couldn't find builtin strategy %s, couldn't load file-based strategy %s", strategy, strategy)
				}
				strat = string(fileStrategy)
			}
			cfg, err := Run(args[0], marshaller, strat, cmd.Context())
			if err != nil {
				log.Fatal(err)
			}
			fmt.Print(cfg)
		},
	}
	var supportedStrategyNames []string
	for s := range strategies {
		supportedStrategyNames = append(supportedStrategyNames, s)
	}
	cmd.Flags().StringVarP(&strategy, "strategy", "s", "simple", "The inflate strategy, which configures how to translate a higher-level file into a config. Valid options are: [" + strings.Join(supportedStrategyNames, ",") + "]. If a built-in strategy of that name cannot be found, opm will attempt to load a strategy definition from a file.")
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	return cmd
}

func Run(path string, marshaller MarshallFunc, strategy string, ctx context.Context) (string, error) {
	dir, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", err
	}
	is := load.Instances([]string{"."}, &load.Config{
		Dir:       dir,
		Package:   "_",
		DataFiles: true,
		ModuleRoot: dir,
	})
	c := cuecontext.New()
	f, err := parser.ParseFile("strategy.cue", strategy)
	if err != nil {
		return "", err
	}
	generator := c.BuildFile(f)

	datapath := cue.ParsePath("data")
	datavalue := generator.LookupPath(datapath)

	for _, i := range is {
		if i.Err != nil {
			return "", fmt.Errorf("error loading %s: %v", i.Dir, i.Err)
		}
		// handle input json or yaml
		for _, o := range i.OrphanedFiles {
			if o.ExcludeReason != nil {
				continue
			}
			if o.Interpretation != "auto" {
				continue
			}
			if o.Encoding == build.YAML {
				file, err := goyaml.Extract(o.Filename, o.Source)
				if err != nil {
					return "", err
				}
				generator = generator.FillPath(datapath, c.BuildFile(file, cue.Scope(datavalue)))
			}
			if o.Encoding == build.JSON {
				data, err := os.ReadFile(o.Filename)
				if err != nil {
					return "", err
				}
				file, err := gojson.Extract(o.Filename, data)
				if err != nil {
					return "", err
				}
				generator = generator.FillPath(datapath, c.BuildExpr(file, cue.Scope(datavalue)))
			}
		}
		// handle input cue files
		for _, o := range i.Files {
			f := c.BuildFile(o, cue.Scope(datavalue))
			generator = generator.FillPath(datapath, f)
		}
	}
	out, err := marshaller(generator.LookupPath(cue.ParsePath("out")))
	if err != nil {
		// if there's an error marshalling, print the whole object or the error from the whole object
		return generator.String()
	}
	return out, nil
}
