# Operator Bundle

An `Operator Bundle` is a container image that stores the Kubernetes manifests and metadata associated with an operator. A bundle is meant to represent a *specific* version of an operator.

## Operator Bundle Overview

The operator manifests refer to a set of Kubernetes manifest(s) that defines the deployment and RBAC model of the operator. The operator metadata on the other hand are, but not limited to the following properties:

* Information that identifies the operator, its name, version etc.
* Additional information that drives the UI:
  * Icon
  * Example CR(s)
* Channel(s)
* API(s) provided and required.
* Related images.

An `Operator Bundle` is built as a scratch (i.e. non-runnable) container image that contains operator manifests and specific metadata in designated directories inside the image. Then, it can be pushed and pulled from an OCI-compliant container registry. Ultimately, an operator bundle will be used by [Operator Registry](https://github.com/operator-framework/operator-registry) and [Operator-Lifecycle-Manager (OLM)](https://github.com/operator-framework/operator-lifecycle-manager) to install an operator in OLM-enabled clusters.

### Bundle Manifest Format

The standard bundle format requires two directories named `manifests` and `metadata`. The `manifests` directory is where all operator manifests are resided including the `ClusterServiceVersion` (CSV), `CustomResourceDefinition` (CRD) and other supported Kubernetes types. The `metadata` directory is where operator metadata is located including `annotations.yaml` which contains additional information such as the package name, channels and media type. Also, `dependencies.yaml`, which contains the operator dependency information can be included in `metadata` directory.

Below is the directory layout of an example operator bundle inside a bundle image:

```bash
$ tree
/
├── manifests
│   ├── etcdcluster.crd.yaml
│   └── etcdoperator.clusterserviceversion.yaml
└── metadata
    ├── annotations.yaml
    └── dependencies.yaml
```

*Notes:*

* The names of manifests and metadata directories must match the bundle annotations that are specified in `annotations.yaml` file. Currently, those names are set to `manifests` and `metadata`.

### Bundle Annotations

We use the following labels to annotate the operator bundle image:

* The label `operators.operatorframework.io.bundle.mediatype.v1` reflects the media type or format of the operator bundle. It could be helm charts, plain Kubernetes manifests, etc.
* The label `operators.operatorframework.io.bundle.manifests.v1` reflects the path in the image to the directory that contains the operator manifests. This label is reserved for the future use and is set to `manifests/` for the time being.
* The label `operators.operatorframework.io.bundle.metadata.v1` reflects the path in the image to the directory that contains metadata files about the bundle. This label is reserved for the future use and is set to `metadata/` for the time being.
* The `manifests.v1` and `metadata.v1` labels imply the bundle type:
  * The value `manifests.v1` implies that this bundle contains operator manifests.
  * The value `metadata.v1` implies that this bundle has operator metadata.
* The label `operators.operatorframework.io.bundle.package.v1` reflects the package name of the bundle.
* The label `operators.operatorframework.io.bundle.channels.v1` reflects the list of channels the bundle is subscribing to when added into an operator registry.
* The label `operators.operatorframework.io.bundle.channel.default.v1` reflects the default channel an operator should be subscribed to when installed from a registry. This label is optional if the default channel has been set by previous bundles and the default channel is unchanged for this bundle.

The labels will also be put inside a YAML file, `annotations.yaml`, as shown below:

```yaml
annotations:
  operators.operatorframework.io.bundle.mediatype.v1: "registry+v1"
  operators.operatorframework.io.bundle.manifests.v1: "manifests/"
  operators.operatorframework.io.bundle.metadata.v1: "metadata/"
  operators.operatorframework.io.bundle.package.v1: "test-operator"
  operators.operatorframework.io.bundle.channels.v1: "beta,stable"
  operators.operatorframework.io.bundle.channel.default.v1: "stable"
```

*Notes:*

* In case of a mismatch, the `annotations.yaml` file is authoritative because the on-cluster operator-registry that relies on these annotations has access to the yaml file only.
* The potential use case for the `LABELS` is - an external off-cluster tool can inspect the image to check the type of a given bundle image without downloading the content.
* The annotations for bundle manifests and metadata are reserved for future use. They are set to be `manifests/` and `metadata/` for the time being.

### Bundle Dependencies

The dependencies of an operator are listed as a list in `dependencies.yaml` file inside `/metadata` folder of a bundle. This file is optional and only used to specify explicit operator version dependencies at first. Eventually, operator authors can migrate the API-based dependencies into `dependencies.yaml` as well in the future. The ultimate goal is to have `dependencies.yaml` as a centralized metadata for operator dependencies and moving the dependency information away from CSV.

The dependency list will contain a `type` field for each item to specify what kind of dependency this is. There are two supported `type` of operator dependencies. It can be a package type (`olm.package`) meaning this is a dependency for a specific operator version. For `olm.package` type, the dependency information should include the `package` name and the `version` of the package in semver format. We use `blang/semver` library for semver parsing (https://github.com/blang/semver). For example, you can specify an exact version such as `0.5.2` or a range of version such as `>0.5.1` (https://github.com/blang/semver#ranges). In addition, the author can specify dependency that is similar to existing CRD/API-based using `olm.gvk` type and then specify GVK information as how it is done in CSV. This is a path to enable operator authors to consolidate all dependencies (API or explicit version) to be in the same place.

An example of a `dependencies.yaml` that specifies Prometheus operator and etcd CRD dependencies:

```yaml
dependencies:
  - type: olm.package
    value:
      packageName: prometheus
      version: ">0.27.0"
  - type: olm.gvk
    value:
      group: etcd.database.coreos.com
      kind: EtcdCluster
      version: v1beta2
```

### Bundle Dockerfile

This is an example of a `Dockerfile` for operator bundle:

```dockerfile
FROM scratch

# We are pushing an operator-registry bundle
# that has both metadata and manifests.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=test-operator
LABEL operators.operatorframework.io.bundle.channels.v1=beta,stable
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable

ADD test/*.yaml /manifests/
ADD test/metadata/annotations.yaml /metadata/annotations.yaml
```

## Operator Bundle Commands

`opm` (Operator Package Manager) is a CLI tool to generate bundle annotations, build bundle manifests image, validate bundle manifests image and other functionalities. Please note that the `generate`, `build` and `validate` features of `opm` CLI are currently in alpha and only meant for development use.

### `opm` (Operator Package Manager)

In order to use `opm` CLI, follow the `opm` build instruction:

1. Clone the operator registry repository:

```bash
git clone https://github.com/operator-framework/operator-registry
```

2. Build `opm` binary using this command:

```bash
make build
```

Now, a binary named `opm` is now built in current directory and ready to be used.
