module github.com/operator-framework/operator-registry

go 1.16

require (
	github.com/Microsoft/hcsshim v0.8.9 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/bugsnag/bugsnag-go v1.5.3 // indirect
	github.com/bugsnag/panicwrap v1.2.0 // indirect
	github.com/containerd/containerd v1.4.8
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
	github.com/golang/mock v1.4.1
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/gorilla/handlers v1.4.2 // indirect
	github.com/grpc-ecosystem/grpc-health-probe v0.3.2
	github.com/h2non/filetype v1.1.1
	github.com/h2non/go-is-svg v0.0.0-20160927212452-35e8c4b0612c
	github.com/joelanford/ignore v0.0.0-20210607151042-0d25dc18b62d
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.2
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/operator-framework/api v0.10.5
	github.com/otiai10/copy v1.2.0
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.7 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	go.etcd.io/bbolt v1.3.6
	golang.org/x/mod v0.4.2
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.38.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v0.0.0-20200709232328-d8193ee9cc3e
	google.golang.org/protobuf v1.26.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.0
	k8s.io/apiextensions-apiserver v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/kubectl v0.22.0
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/yaml v1.2.0
)

// latest tag resolves to a very old version. this is only used for spinning up local test registries
replace github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
