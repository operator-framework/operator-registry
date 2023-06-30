package action

import (
	"fmt"
	"io"
	"text/template"
)

type GenerateDevfile struct {
	IndexDir string
	Name     string
	BuildCTX string
	Provider string
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
  name: {{.Name}}
  displayName: {{.Name}}
  description: 'File based catalog'
  language: fbc
  provider: {{.Provider}}
components:
  - name: image-build
    image:
      imageName: fbc:latest
      dockerfile:
        uri: {{.IndexDir}}.Dockerfile
        buildContext: {{.BuildCTX}}
commands:
  - id: build-image
    apply:
      component: image-build
`


// ╰─ cat catalogs.yaml
// schema: olm.composite.catalogs
// catalogs:
// - name: v4.10
//   destination:
//     baseImage: quay.io/operator-framework/opm:v1.24
//     workingDir: catalogs/v4.10
//   builders:
//     - olm.builder.basic
// - name: v4.11
//   destination:
//     baseImage: quay.io/operator-framework/opm:v1.25
//     workingDir: catalogs/v4.11
//   builders:
//     - olm.builder.semver
// - name: v4.12
//   destination:
//     baseImage: quay.io/operator-framework/opm:v1.26
//     workingDir: catalogs/v4.12
//   builders:
//     - olm.builder.raw
// - name: v4.13
//   destination:
//     baseImage: quay.io/operator-framework/opm:v1.26
//     workingDir: catalogs/v4.13
//   builders:
//     - olm.builder.custom


// ╭─ ~/devel/fbc-composite-example  main                                                                                                           ✔ 
// ╰─ cat contributions.yaml
// schema: olm.composite
// components:
// - name: v4.10
//   destination:
//     path: my-package
//   strategy:
//     name: basic
//     template:
//       schema: olm.builder.basic
//       config:
//         input: components/v4.10.yaml
//         output: catalog.yaml
// - name: v4.11
//   destination:
//     path: my-package
//   strategy:
//     name: semver
//     template:
//       schema: olm.builder.semver
//       config:
//         input: components/v4.11.yaml
//         output: catalog.yaml
// - name: v4.12
//   destination:
//     path: my-package
//   strategy:
//     name: raw
//     template:
//       schema: olm.builder.raw
//       config:
//         input: components/v4.12.yaml
//         output: catalog.yaml
// - name: v4.13
//   destination:
//     path: my-package
//   strategy:
//     name: custom
//     template:
//       schema: olm.builder.custom
//       config:
//         command: cat
//         args:
//           - "components/v4.13.yaml"
//         output: catalog.yaml