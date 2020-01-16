module github.com/operator-framework/operator-registry

require (
	github.com/antihax/optional v0.0.0-20180407024304-ca021399b1a6
	github.com/docker/distribution v2.7.1+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/golang-migrate/migrate/v4 v4.6.2
	github.com/golang/mock v1.2.0
	github.com/golang/protobuf v1.3.2
	github.com/grpc-ecosystem/grpc-health-probe v0.2.1-0.20181220223928-2bf0a5b182db
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.2
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/otiai10/copy v1.0.1
	github.com/otiai10/curr v0.0.0-20190513014714-f5a3d24e5776 // indirect
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.4.0
	golang.org/x/net v0.0.0-20191028085509-fe3aa8a45271
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	google.golang.org/grpc v1.24.0
	gopkg.in/yaml.v2 v2.2.4
	helm.sh/helm/v3 v3.0.1
	k8s.io/api v0.0.0-20191016110408-35e52d86657a
	k8s.io/apiextensions-apiserver v0.0.0-20191016113550-5357c4baaf65
	k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/client-go v0.0.0-20191016111102-bec269661e48
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.0.0-20191016120415-2ed914427d51
)

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm

go 1.13
