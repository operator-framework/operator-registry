package converter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/alpha/template/basic"
	"github.com/operator-framework/operator-registry/alpha/template/substitutes"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type Converter struct {
	FbcReader               io.Reader
	OutputFormat            string
	Registry                image.Registry
	DestinationTemplateType string // TODO: when we have a template factory, we can pass it here
}

func (c *Converter) Convert() error {
	var b []byte
	var err error
	switch c.DestinationTemplateType {
	case "basic":
		var bt *basic.BasicTemplateData
		bt, err = basic.FromReader(c.FbcReader)
		if err != nil {
			return err
		}
		b, err = json.MarshalIndent(bt, "", "    ")
		if err != nil {
			return err
		}
	case "substitutes":
		var st *substitutes.SubstitutesTemplateData
		st, err = substitutes.FromReader(c.FbcReader)
		if err != nil {
			return err
		}
		b, err = json.MarshalIndent(st, "", "    ")
		if err != nil {
			return err
		}
	default:
		// usage pattern prevents us from getting here, so if we do it's a programmer failure and we should panic
		panic(fmt.Sprintf("unknown template type %q", c.DestinationTemplateType))
	}

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
