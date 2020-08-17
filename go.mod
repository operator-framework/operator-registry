module github.com/operator-framework/operator-registry

go 1.13

require (
	github.com/Microsoft/hcsshim v0.8.9 // indirect
	github.com/antihax/optional v0.0.0-20180407024304-ca021399b1a6
	github.com/blang/semver v3.5.0+incompatible
	github.com/bugsnag/bugsnag-go v1.5.3 // indirect
	github.com/bugsnag/panicwrap v1.2.0 // indirect
	github.com/containerd/containerd v1.3.2
	github.com/containerd/continuity v0.0.0-20200413184840-d3ef23f19fbb // indirect
	github.com/containerd/ttrpc v1.0.1 // indirect
	github.com/docker/cli v0.0.0-20200130152716-5d0cf8839492
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/garyburd/redigo v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/gofrs/uuid v3.3.0+incompatible // indirect
	github.com/golang-migrate/migrate/v4 v4.6.2
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.0
	github.com/gorilla/handlers v1.4.2 // indirect
	github.com/grpc-ecosystem/grpc-health-probe v0.3.2
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.2
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/operator-framework/api v0.3.7-0.20200602203552-431198de9fc2
	github.com/otiai10/copy v1.2.0
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.5.1
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.7 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	go.etcd.io/bbolt v1.3.4
	golang.org/x/mod v0.2.0
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/genproto v0.0.0-20200701001935-0939c5918c31 // indirect
	google.golang.org/grpc v1.30.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v0.0.0-20200709232328-d8193ee9cc3e
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.2
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.18.0
)

replace (
	// Currently on a fork for two issues:
	// 1. stage registry proxy didn't like requests with no scopes, see https://github.com/containerd/containerd/pull/4223
	// 2. prod registry proxy returns a 403 on post, see https://github.com/containerd/containerd/pull/3913
	// The fork can be removed when both issues are resolved in a release, which should be 1.4.0
	github.com/containerd/containerd => github.com/ecordell/containerd v1.3.1-0.20200629153125-0ff1a1be2fa5

	// latest tag resolves to a very old version. this is only used for spinning up local test registries
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
)
