package containertools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCommand(t *testing.T) {
	for _, tt := range []struct {
		Name    string
		Factory CommandFactory
		Options BuildOptions
		Args    []string
	}{
		{
			Name:    "docker defaults",
			Factory: &DockerCommandFactory{},
			Options: DefaultBuildOptions(),
			Args: []string{
				"docker", "build", ".",
			},
		},
		{
			Name:    "docker unsupported format",
			Factory: &DockerCommandFactory{},
			Options: BuildOptions{
				context: ".",
				format:  "oci",
			},
		},
		{
			Name:    "docker context",
			Factory: &DockerCommandFactory{},
			Options: BuildOptions{
				context: "foo",
			},
			Args: []string{
				"docker", "build", "foo",
			},
		},
		{
			Name:    "docker dockerfile",
			Factory: &DockerCommandFactory{},
			Options: BuildOptions{
				context:    ".",
				dockerfile: "foo",
			},
			Args: []string{
				"docker", "build", "-f", "foo", ".",
			},
		},
		{
			Name:    "docker single tag",
			Factory: &DockerCommandFactory{},
			Options: BuildOptions{
				context: ".",
				tags:    []string{"foo"},
			},
			Args: []string{
				"docker", "build", "-t", "foo", ".",
			},
		},
		{
			Name:    "docker multiple tags",
			Factory: &DockerCommandFactory{},
			Options: BuildOptions{
				context: ".",
				tags:    []string{"foo", "bar"},
			},
			Args: []string{
				"docker", "build", "-t", "foo", "-t", "bar", ".",
			},
		},
		{
			Name:    "podman defaults",
			Factory: &PodmanCommandFactory{},
			Options: DefaultBuildOptions(),
			Args: []string{
				"podman", "build", "--format", "docker", "--tls-verify=false", ".",
			},
		},
		{
			Name:    "podman oci format",
			Factory: &PodmanCommandFactory{},
			Options: BuildOptions{
				context: ".",
				format:  "oci",
			},
			Args: []string{
				"podman", "build", "--format", "oci", "--tls-verify=false", ".",
			},
		},
		{
			Name:    "podman context",
			Factory: &PodmanCommandFactory{},
			Options: BuildOptions{
				context: "foo",
			},
			Args: []string{
				"podman", "build", "--format", "docker", "--tls-verify=false", "foo",
			},
		},
		{
			Name:    "podman dockerfile",
			Factory: &PodmanCommandFactory{},
			Options: BuildOptions{
				context:    ".",
				dockerfile: "foo",
			},
			Args: []string{
				"podman", "build", "--format", "docker", "-f", "foo", "--tls-verify=false", ".",
			},
		},
		{
			Name:    "podman single tag",
			Factory: &PodmanCommandFactory{},
			Options: BuildOptions{
				context: ".",
				tags:    []string{"foo"},
			},
			Args: []string{
				"podman", "build", "--format", "docker", "-t", "foo", "--tls-verify=false", ".",
			},
		},
		{
			Name:    "podman multiple tags",
			Factory: &PodmanCommandFactory{},
			Options: BuildOptions{
				context: ".",
				tags:    []string{"foo", "bar"},
			},
			Args: []string{
				"podman", "build", "--format", "docker", "-t", "foo", "-t", "bar", "--tls-verify=false", ".",
			},
		},
		{
			Name:    "stub defaults",
			Factory: &StubCommandFactory{},
			Options: DefaultBuildOptions(),
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			require := require.New(t)

			cmd, err := tt.Factory.BuildCommand(tt.Options)
			if tt.Args == nil {
				require.Nil(cmd)
				require.Error(err)
			} else {
				require.NotNil(cmd)
				require.NoError(err)
				require.Equal(tt.Args, cmd.Args)
			}
		})
	}
}
