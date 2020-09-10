package containerdregistry

import (
	"time"

	"github.com/opencontainers/go-digest"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/operator-framework/operator-registry/pkg/image"
)

type layer struct {
	diffID     digest.Digest
	descriptor *ocispecv1.Descriptor
}

// PortType is used to specify an exposed port's protocol. By default, this is TCP.
type PortType string

const (
	// TCPType is used to expose ports as TCP ports. This is also the default behavior for an empty PortType
	TCPType PortType = "tcp"
	// UDPType is used to expose ports as UDP ports.
	UDPType PortType = "udp"
)

// BuildConfig is used to create and update image manifests and configs
type BuildConfig struct {
	OmitTimestamp     bool
	CreationTimestamp *time.Time
	Author            *string
	Comment           *string
	Layers            []layer
	Labels            map[string]string
	Ports             map[int][]PortType
	Env               []string
	StopSignal        *string
	Volumes           map[string]*struct{}
	WorkingDir        *string
	User              *string
	Entrypoint        *[]string
	Cmd               *[]string
	Platform          *ocispecv1.Platform
	SquashLayers      bool
	BaseImage         image.Reference
}

// BuildOpt provides a set of options that can be used in image manifest and config updates
type BuildOpt func(config *BuildConfig)

// DefaultBuildConfig provides an empty BuildConfig
func DefaultBuildConfig() *BuildConfig {
	return &BuildConfig{
		BaseImage: image.SimpleReference(emptyBaseImage),
	}
}

// WithAuthor sets the author for for the current operation
func WithAuthor(author string) BuildOpt {
	return func(config *BuildConfig) {
		config.Author = &author
	}
}

// AddVolumes adds volumes to the image
func AddVolumes(volumes []string) BuildOpt {
	return func(config *BuildConfig) {
		for _, v := range volumes {
			config.Volumes[v] = &struct{}{}
		}
	}
}

// RemoveVolumes removes the specified volumes from the image if they exist.
func RemoveVolumes(volumes []string) BuildOpt {
	return func(config *BuildConfig) {
		for _, v := range volumes {
			config.Volumes[v] = nil
		}
	}
}

// WithStopSignal sets a StopSignal for the image. This can be an unsigned signal number or a signal name like SIGKILL
func WithStopSignal(sig string) BuildOpt {
	return func(config *BuildConfig) {
		config.StopSignal = &sig
	}
}

// WithComment adds a custom comment to the image history entry for the current operation
func WithComment(comment string) BuildOpt {
	return func(config *BuildConfig) {
		config.Comment = &comment
	}
}

// OmitTimestamp sets the timestamp for current operation to a zero value.
func OmitTimestamp() BuildOpt {
	return func(config *BuildConfig) {
		config.OmitTimestamp = true
		config.CreationTimestamp = nil
	}
}

// WithTimestamp sets a custom timestamp for the current operation
// If OmitTimestamp is specified, this value will be ignored
func WithTimestamp(creationTimestamp *time.Time) BuildOpt {
	return func(config *BuildConfig) {
		config.CreationTimestamp = creationTimestamp
		config.OmitTimestamp = false
	}
}

// WithLabels adds labels to the image. A 0 length label will delete that label from the image
func WithLabels(labels map[string]string) BuildOpt {
	return func(config *BuildConfig) {
		for k, v := range labels {
			config.Labels[k] = v
		}
	}
}

// WithEnv adds environment variables to the image
// This is additive, preexisting env variables on the image will be preserved.
func WithEnv(env []string) BuildOpt {
	return func(config *BuildConfig) {
		config.Env = append(config.Env, env...)
	}
}

// addLayer adds a layer to the image with its descriptor and diffID.
// This function does not perform any validation on the given descriptor or diffID
func addLayer(desc *ocispecv1.Descriptor, diffID digest.Digest) BuildOpt {
	return func(config *BuildConfig) {
		config.Layers = append(config.Layers, layer{
			descriptor: desc,
			diffID:     diffID,
		})
	}
}

// WithWorkingDir sets the working directory within the image
func WithWorkingDir(wd string) BuildOpt {
	return func(config *BuildConfig) {
		config.WorkingDir = &wd
	}
}

// WithUser sets the User within the image
func WithUser(user string) BuildOpt {
	return func(config *BuildConfig) {
		config.User = &user
	}
}

// WithEntrypoint sets the Entrypoint on the image
func WithEntrypoint(entrypoint []string) BuildOpt {
	return func(config *BuildConfig) {
		config.Entrypoint = &entrypoint
	}
}

// WithCmd sets the CMD on the image
func WithCmd(cmd []string) BuildOpt {
	return func(config *BuildConfig) {
		config.Cmd = &cmd
	}
}

// ExposePorts adds ports to the ExposedPorts list on the image
func ExposePorts(ports []int, proto PortType) BuildOpt {
	return func(config *BuildConfig) {
		for _, p := range ports {
			if config.Ports[p] == nil {
				config.Ports[p] = make([]PortType, 0)
			}
			config.Ports[p] = append(config.Ports[p], proto)
		}
	}
}

// UnexposePorts removes ports from the ExposedPorts list on the image
func UnexposePorts(ports []int) BuildOpt {
	return func(config *BuildConfig) {
		for _, p := range ports {
			config.Ports[p] = nil
		}
	}
}

// WithPlatform sets the platform on a newly created image. This option is only used
// if the image either has 'scratch' or an empty base image, or has a base image with an empty config.
func WithPlatform(p ocispecv1.Platform) BuildOpt {
	return func(config *BuildConfig) {
		config.Platform = &p
	}
}

// MergeLayers creates a single layer image from the current image root
func SquashLayers() BuildOpt {
	return func(config *BuildConfig) {
		config.SquashLayers = true
	}
}

// WithBaseImage sets a base image to pull when building the new image
func WithBaseImage(img image.Reference) BuildOpt {
	return func(config *BuildConfig) {
		config.BaseImage = img
	}
}
