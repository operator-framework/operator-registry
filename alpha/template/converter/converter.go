package converter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/alpha/template/basic"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type Converter struct {
	FbcReader    io.Reader
	OutputFormat string
	Registry     image.Registry
}

func (c *Converter) Convert() error {
	bt, err := basic.FromReader(c.FbcReader)
	if err != nil {
		return err
	}

	b, _ := json.MarshalIndent(bt, "", "    ")
	if c.OutputFormat == "json" {
		fmt.Fprintln(os.Stdout, string(b))
	} else {
		y, err := yaml.JSONToYAML(b)
		if err != nil {
			return err
		}
		y = append([]byte("---\n"), y...)
		fmt.Fprintln(os.Stdout, string(y))
	}

	return nil
}
