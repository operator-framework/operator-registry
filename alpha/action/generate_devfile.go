package action

import (
	"fmt"
	"io"
	"text/template"
)

type GenerateDevfile struct {
	IndexDir string
	Writer   io.Writer
}

func (i GenerateDevfile) Run() error {
	if err := i.validate(); err != nil {
		return err
	}

	t, err := template.New("devfile").Parse(devfileTmpl)
	if err != nil {
		// The template is hardcoded in the binary, so if
		// there is a parse error, it was a programmer error.
		panic(err)
	}
	return t.Execute(i.Writer, i)
}

func (i GenerateDevfile) validate() error {
	if i.IndexDir == "" {
		return fmt.Errorf("index directory is unset")
	}
	return nil
}

const devfileTmpl = `schemaVersion: 2.2.0
metadata:
  name: fbc-4.11
  displayName: FBC 4.11
  description: 'File based catalog'
  language: fbc
  provider: Red Hat
components:
  - name: image-build
    image:
      imageName: fbc:latest
      dockerfile:
        uri: {{.IndexDir}}.Dockerfile
        buildContext: "4.11"
commands:
  - id: build-image
    apply:
      component: image-build
`
