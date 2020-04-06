module github.com/operator-framework/operator-registry

go 1.13

require (
	github.com/Microsoft/hcsshim v0.8.7 // indirect
	github.com/antihax/optional v0.0.0-20180407024304-ca021399b1a6
	github.com/blang/semver v3.5.0+incompatible
	github.com/containerd/containerd v1.3.2
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c // indirect
	github.com/docker/cli v0.0.0-20200130152716-5d0cf8839492
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/golang-migrate/migrate/v4 v4.6.2
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/grpc-ecosystem/grpc-health-probe v0.2.1-0.20181220223928-2bf0a5b182db
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/runc v1.0.0-rc9 // indirect
	github.com/opencontainers/runtime-spec v0.1.2-0.20190618234442-a950415649c7 // indirect
	github.com/operator-framework/api v0.1.1
	github.com/otiai10/copy v1.0.2
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.5.1
	go.etcd.io/bbolt v1.3.4
	golang.org/x/mod v0.2.0
	golang.org/x/net v0.0.0-20191028085509-fe3aa8a45271
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	google.golang.org/grpc v1.26.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.0
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/client-go v0.18.0
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.18.0
)

replace github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
