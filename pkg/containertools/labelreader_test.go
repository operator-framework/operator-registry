package containertools_test

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/containertools/containertoolsfakes"
)

func TestReadDockerLabels(t *testing.T) {
	image := "quay.io/operator-framework/example"
	imageData := exampleInspectResultDocker
	expectedLabelKey := "operators.operatorframework.io.index.database.v1"
	expectedLabelVal := "./index.db"
	containerTool := "docker"

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(nil)
	mockCmd.InspectReturns([]byte(imageData), nil)
	mockCmd.GetToolNameReturns(containerTool)

	labelReader := containertools.ImageLabelReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	labels, err := labelReader.GetLabelsFromImage(image)
	require.NoError(t, err)
	require.Equal(t, labels[expectedLabelKey], expectedLabelVal)
}

func TestReadDockerLabelsNoLabels(t *testing.T) {
	image := "quay.io/operator-framework/example"
	imageData := exampleInspectResultDockerNoLabels
	containerTool := "docker"

	mockCmd := containertoolsfakes.FakeCommandRunner{}

	logger := logrus.NewEntry(logrus.New())

	labelReader := containertools.ImageLabelReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	mockCmd.PullReturns(nil)
	mockCmd.InspectReturns([]byte(imageData), nil)
	mockCmd.GetToolNameReturns(containerTool)

	labels, err := labelReader.GetLabelsFromImage(image)
	require.NoError(t, err)
	require.Equal(t, len(labels), 0)
}

func TestReadPodmanLabels(t *testing.T) {
	image := "quay.io/operator-framework/example"
	imageData := exampleInspectResultPodman
	expectedLabelKey := "operators.operatorframework.io.index.database.v1"
	expectedLabelVal := "./index.db"
	containerTool := "podman"

	mockCmd := containertoolsfakes.FakeCommandRunner{}

	logger := logrus.NewEntry(logrus.New())

	labelReader := containertools.ImageLabelReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	mockCmd.PullReturns(nil)
	mockCmd.InspectReturns([]byte(imageData), nil)
	mockCmd.GetToolNameReturns(containerTool)

	labels, err := labelReader.GetLabelsFromImage(image)
	require.NoError(t, err)
	require.Equal(t, labels[expectedLabelKey], expectedLabelVal)
}

func TestReadPodmanLabelsNoLabels(t *testing.T) {
	image := "quay.io/operator-framework/example"
	imageData := exampleInspectResultPodmanNoLabels
	containerTool := "podman"

	mockCmd := containertoolsfakes.FakeCommandRunner{}

	logger := logrus.NewEntry(logrus.New())

	labelReader := containertools.ImageLabelReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	mockCmd.PullReturns(nil)
	mockCmd.InspectReturns([]byte(imageData), nil)
	mockCmd.GetToolNameReturns(containerTool)

	labels, err := labelReader.GetLabelsFromImage(image)
	require.NoError(t, err)
	require.Equal(t, len(labels), 0)
}

func TestReadDockerLabels_PullError(t *testing.T) {
	image := "quay.io/operator-framework/example"
	pullErr := fmt.Errorf("Error pulling image")

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(pullErr)

	labelReader := containertools.ImageLabelReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	_, err := labelReader.GetLabelsFromImage(image)
	require.Error(t, err)
	require.EqualError(t, err, pullErr.Error())
}

func TestReadDockerLabels_InspectError(t *testing.T) {
	image := "quay.io/operator-framework/example"
	containerTool := "docker"
	inspectErr := fmt.Errorf("Error inspecting image")

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(nil)
	mockCmd.InspectReturns(nil, inspectErr)
	mockCmd.GetToolNameReturns(containerTool)

	labelReader := containertools.ImageLabelReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	_, err := labelReader.GetLabelsFromImage(image)
	require.Error(t, err)
	require.EqualError(t, err, inspectErr.Error())
}

func TestReadDockerLabels_InvalidData_Error(t *testing.T) {
	image := "quay.io/operator-framework/example"
	imageData := "invalidJson"
	containerTool := "docker"

	logger := logrus.NewEntry(logrus.New())
	mockCmd := containertoolsfakes.FakeCommandRunner{}

	mockCmd.PullReturns(nil)
	mockCmd.InspectReturns([]byte(imageData), nil)
	mockCmd.GetToolNameReturns(containerTool)

	labelReader := containertools.ImageLabelReader{
		Cmd:    &mockCmd,
		Logger: logger,
	}

	_, err := labelReader.GetLabelsFromImage(image)
	require.Error(t, err)
}

const exampleInspectResultDocker = `
[
    {
        "Id": "sha256:2fdf50f894c2619bf65b558786361ded7207e5484ba390ed138c481600fcf36b",
        "RepoTags": [
            "quay.io/operator-framework/example:latest"
        ],
        "RepoDigests": [],
        "Parent": "sha256:db6bd804f8e64a15b88ad73fb384d16db99c994fe1b8e911173504823f5edfd8",
        "Comment": "",
        "Created": "2019-10-24T13:52:26.78433621Z",
        "Container": "bd69c19fc3f1e4a1a15710a905d1fe229abd2e22c9920f48c66dbdf293955707",
        "ContainerConfig": {
            "Hostname": "bd69c19fc3f1",
            "Domainname": "",
            "User": "",
            "AttachStdin": false,
            "AttachStdout": false,
            "AttachStderr": false,
            "ExposedPorts": {
                "50051/tcp": {}
            },
            "Tty": false,
            "OpenStdin": false,
            "StdinOnce": false,
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Cmd": [
                "/bin/sh",
                "-c",
                "#(nop) ",
                "CMD [\"registry\" \"serve\" \"--database\" \"bundles.db\"]"
            ],
            "ArgsEscaped": true,
            "Image": "sha256:db6bd804f8e64a15b88ad73fb384d16db99c994fe1b8e911173504823f5edfd8",
            "Volumes": null,
            "WorkingDir": "",
            "Entrypoint": [
                "/opm"
            ],
            "OnBuild": null,
            "Labels": {
                "operators.operatorframework.io.index.database.v1": "./index.db"
            }
        },
        "DockerVersion": "18.09.6",
        "Author": "",
        "Config": {
            "Hostname": "",
            "Domainname": "",
            "User": "",
            "AttachStdin": false,
            "AttachStdout": false,
            "AttachStderr": false,
            "ExposedPorts": {
                "50051/tcp": {}
            },
            "Tty": false,
            "OpenStdin": false,
            "StdinOnce": false,
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Cmd": [
                "registry",
                "serve",
                "--database",
                "bundles.db"
            ],
            "ArgsEscaped": true,
            "Image": "sha256:db6bd804f8e64a15b88ad73fb384d16db99c994fe1b8e911173504823f5edfd8",
            "Volumes": null,
            "WorkingDir": "",
            "Entrypoint": [
                "/opm"
            ],
            "OnBuild": null,
            "Labels": {
                "operators.operatorframework.io.index.database.v1": "./index.db"
            }
        },
        "Architecture": "amd64",
        "Os": "linux",
        "Size": 34964195,
        "VirtualSize": 34964195,
        "GraphDriver": {
            "Data": {
                "LowerDir": "/var/lib/docker/overlay2/564c13b65756d391130ad48c0e56b53aab18420864e04f30aa0ecb3d552c9326/diff",
                "MergedDir": "/var/lib/docker/overlay2/46eba8752f7449218076b1219e7f654cd7ef19cc52e5cdadaf3e09eb871e0f22/merged",
                "UpperDir": "/var/lib/docker/overlay2/46eba8752f7449218076b1219e7f654cd7ef19cc52e5cdadaf3e09eb871e0f22/diff",
                "WorkDir": "/var/lib/docker/overlay2/46eba8752f7449218076b1219e7f654cd7ef19cc52e5cdadaf3e09eb871e0f22/work"
            },
            "Name": "overlay2"
        },
        "RootFS": {
            "Type": "layers",
            "Layers": [
                "sha256:537370cc1e9e78ba5188b9426d2f99b8684716927513277e76c8a6c894ef5bab",
                "sha256:497557b5d53e6020f1579b5b9c5a1006219cbe31a1b2fddc0c79736f62232fe7"
            ]
        },
        "Metadata": {
            "LastTagTime": "2019-10-24T09:52:26.893503009-04:00"
        }
    }
]
`

const exampleInspectResultDockerNoLabels = `
[
    {
        "Id": "sha256:2fdf50f894c2619bf65b558786361ded7207e5484ba390ed138c481600fcf36b",
        "RepoTags": [
            "quay.io/operator-framework/example:latest"
        ],
        "RepoDigests": [],
        "Parent": "sha256:db6bd804f8e64a15b88ad73fb384d16db99c994fe1b8e911173504823f5edfd8",
        "Comment": "",
        "Created": "2019-10-24T13:52:26.78433621Z",
        "Container": "bd69c19fc3f1e4a1a15710a905d1fe229abd2e22c9920f48c66dbdf293955707",
        "ContainerConfig": {
            "Hostname": "bd69c19fc3f1",
            "Domainname": "",
            "User": "",
            "AttachStdin": false,
            "AttachStdout": false,
            "AttachStderr": false,
            "ExposedPorts": {
                "50051/tcp": {}
            },
            "Tty": false,
            "OpenStdin": false,
            "StdinOnce": false,
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Cmd": [
                "/bin/sh",
                "-c",
                "#(nop) ",
                "CMD [\"registry\" \"serve\" \"--database\" \"bundles.db\"]"
            ],
            "ArgsEscaped": true,
            "Image": "sha256:db6bd804f8e64a15b88ad73fb384d16db99c994fe1b8e911173504823f5edfd8",
            "Volumes": null,
            "WorkingDir": "",
            "Entrypoint": [
                "/opm"
            ],
            "OnBuild": null
        },
        "DockerVersion": "18.09.6",
        "Author": "",
        "Config": {
            "Hostname": "",
            "Domainname": "",
            "User": "",
            "AttachStdin": false,
            "AttachStdout": false,
            "AttachStderr": false,
            "ExposedPorts": {
                "50051/tcp": {}
            },
            "Tty": false,
            "OpenStdin": false,
            "StdinOnce": false,
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Cmd": [
                "registry",
                "serve",
                "--database",
                "bundles.db"
            ],
            "ArgsEscaped": true,
            "Image": "sha256:db6bd804f8e64a15b88ad73fb384d16db99c994fe1b8e911173504823f5edfd8",
            "Volumes": null,
            "WorkingDir": "",
            "Entrypoint": [
                "/opm"
            ],
            "OnBuild": null
        },
        "Architecture": "amd64",
        "Os": "linux",
        "Size": 34964195,
        "VirtualSize": 34964195,
        "GraphDriver": {
            "Data": {
                "LowerDir": "/var/lib/docker/overlay2/564c13b65756d391130ad48c0e56b53aab18420864e04f30aa0ecb3d552c9326/diff",
                "MergedDir": "/var/lib/docker/overlay2/46eba8752f7449218076b1219e7f654cd7ef19cc52e5cdadaf3e09eb871e0f22/merged",
                "UpperDir": "/var/lib/docker/overlay2/46eba8752f7449218076b1219e7f654cd7ef19cc52e5cdadaf3e09eb871e0f22/diff",
                "WorkDir": "/var/lib/docker/overlay2/46eba8752f7449218076b1219e7f654cd7ef19cc52e5cdadaf3e09eb871e0f22/work"
            },
            "Name": "overlay2"
        },
        "RootFS": {
            "Type": "layers",
            "Layers": [
                "sha256:537370cc1e9e78ba5188b9426d2f99b8684716927513277e76c8a6c894ef5bab",
                "sha256:497557b5d53e6020f1579b5b9c5a1006219cbe31a1b2fddc0c79736f62232fe7"
            ]
        },
        "Metadata": {
            "LastTagTime": "2019-10-24T09:52:26.893503009-04:00"
        }
    }
]
`

const exampleInspectResultPodman = `
[
    {
        "Id": "2fdf50f894c2619bf65b558786361ded7207e5484ba390ed138c481600fcf36b",
        "Digest": "sha256:00d9a846550c539c725f35fb088230c229dd40d7707bd9eff33afb43a32f3973",
        "RepoTags": [
            "quay.io/operator-framework/added-to-index:latest"
        ],
        "RepoDigests": [
            "quay.io/operator-framework/added-to-index@sha256:00d9a846550c539c725f35fb088230c229dd40d7707bd9eff33afb43a32f3973"
        ],
        "Parent": "",
        "Comment": "",
        "Created": "2019-10-24T13:52:26.78433621Z",
        "Config": {
            "ExposedPorts": {
                "50051/tcp": {}
            },
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Entrypoint": [
                "/opm"
            ],
            "Cmd": [
                "registry",
                "serve",
                "--database",
                "bundles.db"
            ],
            "Labels": {
                "operators.operatorframework.io.index.database.v1": "./index.db"
            }
        },
        "Version": "18.09.6",
        "Author": "",
        "Architecture": "amd64",
        "Os": "linux",
        "Size": 34978670,
        "VirtualSize": 34978670,
        "GraphDriver": {
            "Name": "overlay",
            "Data": {
                "LowerDir": "/home/krizza/.local/share/containers/storage/overlay/537370cc1e9e78ba5188b9426d2f99b8684716927513277e76c8a6c894ef5bab/diff",
                "UpperDir": "/home/krizza/.local/share/containers/storage/overlay/81c08fd4332b5254348e09f27d354b0ce687c0f2ab0841e43e6716aa2fcb60b8/diff",
                "WorkDir": "/home/krizza/.local/share/containers/storage/overlay/81c08fd4332b5254348e09f27d354b0ce687c0f2ab0841e43e6716aa2fcb60b8/work"
            }
        },
        "RootFS": {
            "Type": "layers",
            "Layers": [
                "",
                ""
            ]
        },
        "Labels": {
            "operators.operatorframework.io.index.database.v1": "./index.db"
        },
        "Annotations": {},
        "ManifestType": "application/vnd.docker.distribution.manifest.v1+prettyjws",
        "User": "",
        "History": [
            {
                "created": "2019-10-24T13:52:25.700894949Z",
                "created_by": "/bin/sh -c #(nop)  LABEL operators.operatorframework.io.index.database.v1=./index.db",
                "empty_layer": true
            },
            {
                "created": "2019-10-24T13:52:26.104053162Z",
                "created_by": "/bin/sh -c #(nop) COPY file:c8972101e742e777146e60ca46ffc077a8edb874d78f5395758706041216dab3 in /opm "
            },
            {
                "created": "2019-10-24T13:52:26.33923787Z",
                "created_by": "/bin/sh -c #(nop) COPY file:9501a4e82bb8fa49a1f5b0ba285f0b3f779adbb71346b968b1c4940041ff9c17 in /bin/grpc_health_probe "
            },
            {
                "created": "2019-10-24T13:52:26.493170216Z",
                "created_by": "/bin/sh -c #(nop)  EXPOSE 50051",
                "empty_layer": true
            },
            {
                "created": "2019-10-24T13:52:26.634480886Z",
                "created_by": "/bin/sh -c #(nop)  ENTRYPOINT [\"/opm\"]",
                "empty_layer": true
            },
            {
                "created": "2019-10-24T13:52:26.78433621Z",
                "created_by": "/bin/sh -c #(nop)  CMD [\"registry\" \"serve\" \"--database\" \"bundles.db\"]",
                "empty_layer": true
            }
        ]
    }
]
`

const exampleInspectResultPodmanNoLabels = `
[
    {
        "Id": "2fdf50f894c2619bf65b558786361ded7207e5484ba390ed138c481600fcf36b",
        "Digest": "sha256:00d9a846550c539c725f35fb088230c229dd40d7707bd9eff33afb43a32f3973",
        "RepoTags": [
            "quay.io/operator-framework/added-to-index:latest"
        ],
        "RepoDigests": [
            "quay.io/operator-framework/added-to-index@sha256:00d9a846550c539c725f35fb088230c229dd40d7707bd9eff33afb43a32f3973"
        ],
        "Parent": "",
        "Comment": "",
        "Created": "2019-10-24T13:52:26.78433621Z",
        "Config": {
            "ExposedPorts": {
                "50051/tcp": {}
            },
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Entrypoint": [
                "/opm"
            ],
            "Cmd": [
                "registry",
                "serve",
                "--database",
                "bundles.db"
            ]
        },
        "Version": "18.09.6",
        "Author": "",
        "Architecture": "amd64",
        "Os": "linux",
        "Size": 34978670,
        "VirtualSize": 34978670,
        "GraphDriver": {
            "Name": "overlay",
            "Data": {
                "LowerDir": "/home/krizza/.local/share/containers/storage/overlay/537370cc1e9e78ba5188b9426d2f99b8684716927513277e76c8a6c894ef5bab/diff",
                "UpperDir": "/home/krizza/.local/share/containers/storage/overlay/81c08fd4332b5254348e09f27d354b0ce687c0f2ab0841e43e6716aa2fcb60b8/diff",
                "WorkDir": "/home/krizza/.local/share/containers/storage/overlay/81c08fd4332b5254348e09f27d354b0ce687c0f2ab0841e43e6716aa2fcb60b8/work"
            }
        },
        "RootFS": {
            "Type": "layers",
            "Layers": [
                "",
                ""
            ]
        },
        "Annotations": {},
        "ManifestType": "application/vnd.docker.distribution.manifest.v1+prettyjws",
        "User": "",
        "History": [
            {
                "created": "2019-10-24T13:52:25.700894949Z",
                "created_by": "/bin/sh -c #(nop)  LABEL operators.operatorframework.io.index.database.v1=./index.db",
                "empty_layer": true
            },
            {
                "created": "2019-10-24T13:52:26.104053162Z",
                "created_by": "/bin/sh -c #(nop) COPY file:c8972101e742e777146e60ca46ffc077a8edb874d78f5395758706041216dab3 in /opm "
            },
            {
                "created": "2019-10-24T13:52:26.33923787Z",
                "created_by": "/bin/sh -c #(nop) COPY file:9501a4e82bb8fa49a1f5b0ba285f0b3f779adbb71346b968b1c4940041ff9c17 in /bin/grpc_health_probe "
            },
            {
                "created": "2019-10-24T13:52:26.493170216Z",
                "created_by": "/bin/sh -c #(nop)  EXPOSE 50051",
                "empty_layer": true
            },
            {
                "created": "2019-10-24T13:52:26.634480886Z",
                "created_by": "/bin/sh -c #(nop)  ENTRYPOINT [\"/opm\"]",
                "empty_layer": true
            },
            {
                "created": "2019-10-24T13:52:26.78433621Z",
                "created_by": "/bin/sh -c #(nop)  CMD [\"registry\" \"serve\" \"--database\" \"bundles.db\"]",
                "empty_layer": true
            }
        ]
    }
]
`
