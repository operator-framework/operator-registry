package inflate

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"

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

type MarshallFunc func(value cue.Value) (string, error)

func NewCmd() *cobra.Command {
	var (
		output string
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
			cfg, err := Run(args[0], marshaller, cmd.Context())
			if err != nil {
				log.Fatal(err)
			}
			fmt.Print(cfg)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|yaml)")
	return cmd
}

func Run(path string, marshaller MarshallFunc, ctx context.Context) (string, error) {
	is := load.Instances([]string{"."}, &load.Config{
		Dir:       path,
		Package:   "_",
		DataFiles: true,
	})
	c := cuecontext.New()
	f, err := parser.ParseFile("builtin_config.cue", simple)
	if err != nil {
		return "", err
	}
	generator := c.BuildFile(f)

	datapath := cue.ParsePath("data")
	datavalue := generator.LookupPath(datapath)

	for _, i := range is {
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

	return marshaller(generator.LookupPath(cue.ParsePath("out")))
}
