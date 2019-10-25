module github.com/operator-framework/operator-registry

require (
	github.com/antihax/optional v0.0.0-20180407024304-ca021399b1a6
	github.com/docker/distribution v2.7.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/golang-migrate/migrate/v4 v4.6.2
	github.com/golang/mock v1.2.0
	github.com/golang/protobuf v1.3.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/grpc-ecosystem/grpc-health-probe v0.2.1-0.20181220223928-2bf0a5b182db
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/otiai10/copy v1.0.1
	github.com/otiai10/curr v0.0.0-20190513014714-f5a3d24e5776 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.3.0
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	google.golang.org/grpc v1.23.0
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apiextensions-apiserver v0.0.0-20190918161926-8f644eb6e783
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	k8s.io/klog v0.4.0
)

go 1.13
