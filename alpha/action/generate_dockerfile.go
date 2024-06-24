package action

import (
	"fmt"
	"io"
	"text/template"

	"github.com/operator-framework/operator-registry/pkg/containertools"
)

type GenerateDockerfile struct {
	BaseImage    string
	BuilderImage string
	IndexDir     string
	ExtraLabels  map[string]string
	Writer       io.Writer
}

func (i GenerateDockerfile) Run() error {
	if err := i.validate(); err != nil {
		return err
	}

	t, err := template.New("dockerfile").Parse(dockerfileTmpl)
	if err != nil {
		// The template is hardcoded in the binary, so if
		// there is a parse error, it was a programmer error.
		panic(err)
	}
	return t.Execute(i.Writer, i)
}

func (i GenerateDockerfile) validate() error {
	if i.BaseImage == "" {
		return fmt.Errorf("base image is unset")
	}
	if i.IndexDir == "" {
		return fmt.Errorf("index directory is unset")
	}
	return nil
}

const dockerfileTmpl = `# The builder image is expected to contain
# /bin/opm (with serve subcommand)
FROM {{.BuilderImage}} as builder

# Copy FBC root into image at /configs and pre-populate serve cache
ADD {{.IndexDir}} /configs
RUN ["/bin/opm", "serve", "/configs", "--cache-dir=/tmp/cache", "--cache-only"]

FROM {{.BaseImage}}

{{- if ne .BaseImage "scratch" }}
# The base image is expected to contain
# /bin/opm (with serve subcommand) and /bin/grpc_health_probe

# Configure the entrypoint and command
ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]
{{- else }}
# OLMv0 CatalogSources that use binary-less images must set:
# spec:
#   grpcPodConfig:
#     extractContent:
#       catalogDir: /configs
#       cacheDir: /tmp/cache
{{- end }}

COPY --from=builder /configs /configs
COPY --from=builder /tmp/cache /tmp/cache

# Set FBC-specific label for the location of the FBC root directory
# in the image
LABEL ` + containertools.ConfigsLocationLabel + `=/configs
{{- if .ExtraLabels }}

# Set other custom labels
{{- range $key, $value := .ExtraLabels }}
LABEL "{{ $key }}"="{{ $value }}"
{{- end }}
{{- end }}
`
