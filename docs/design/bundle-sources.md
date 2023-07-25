# Building a file-based catalog from a plain bundle image

> **Warning:** Operator Lifecycle Manager (OLM) v1 features and components are still experimental. Early adopters and contributors should expect breaking changes. The following procedures are not recommended for use on production clusters.

You can build a static collection of arbitrary Kubernetes manifests in the YAML format, or *plain bundle*, and add the image to a file-based catalog (FBC). The experimental `olm.bundle.mediatype` property of the `olm.bundle` schema object differentiates a plain bundle from a regular (`registry+v1`) bundle. You must set the bundle media type property to `plain+v0` to specify a plain bundle.

For more information, see the [Plain Bundle Specification](https://github.com/operator-framework/rukpak/blob/main/docs/bundles/plain.md) in the RukPak repository.

To build a file-based catalog from a plain bundle image, you must complete the following steps:

* Create a plain bundle image
* Create a file-based catalog
* Add the plain bundle image to your file-based catalog
* Build your catalog as an image
* Publish your catalog image

## Building a plain bundle image from an image source

Currently, the Operator Controller only supports installing plain bundles created from a plain bundle image.

<h3 id="prereqs-building-plain-bundle-from-image">
Prerequisites
</h3>

* [`opm` CLI tool](https://github.com/operator-framework/operator-registry/releases)
* Docker or Podman
* Push access to a container registry, such as [Quay](https://quay.io)

<h3 id="proc-building-plain-bundle-from-image">
Procedure
</h3>

1. Verify that your Kubernetes manifests are in a flat directory at the root of your project similar to the following example:

    ```sh
    tree manifests
    manifests
    ├── namespace.yaml
    ├── service_account.yaml
    ├── cluster_role.yaml
    ├── cluster_role_binding.yaml
    └── deployment.yaml
    ```

    * If you are using [kustomize](https://kustomize.io) to build your manifests from templates, you must redirect the output to one or more files under the `manifests/` directory. For example:

        ```sh
        kustomize build templates > manifests/manifests.yaml
        ```

   For more information, see [Building a plain bundle > Prerequisites](https://github.com/operator-framework/rukpak/blob/main/docs/bundles/plain.md#prerequisites).

1. Create a Dockerfile at the root of your project:

    ```sh
    touch plainbundle.Dockerfile
    ```

1. Make the following changes to your Dockerfile:

    *Example Dockerfile*

    ```sh
        FROM scratch
        ADD manifests /manifests
    ```

    > **Note:** Use the `FROM scratch` directive to make the size of the image smaller. No other files or directories are required in the bundle image.

1. Build an OCI-compliant image using your preferred build tool, similar to the following example. You must use an image tag that references a repository where you have push access privileges.

    *Example build command*

    ```sh
    docker build -f plainbundle.Dockerfile -t \
    quay.io/<organization_name>/<repository_name>:<image_tag> .
    ```

1. Push the image to your remote registry:

    ```sh
    docker push quay.io/<organization_name>/<repository_name>:<image_tag>
    ```

### Additional resources

* [File-based catalog bundle schema](https://github.com/operator-framework/olm-docs/blob/master/content/en/docs/Reference/file-based-catalogs.md)
* [OCI image specification](https://github.com/opencontainers/image-spec#oci-image-format-specification)
* [RukPak > Building a plain bundle > Image source](https://github.com/operator-framework/rukpak/blob/main/docs/bundles/plain.md#image-source)
* [RukPak > Sources > Images > Private image registries](https://github.com/operator-framework/rukpak/blob/main/docs/sources/image.md#private-image-registries)

## Creating a file-based catalog

If you do not have a file-based catalog, you must perform the following steps to initialize the catalog.

<h3 id="prereqs-creating-a-fbc">
Prerequisites
</h3>

* `opm` CLI tool
* Docker or Podman

<h3 id="proc-creating-a-fbc">
Procedure
</h3>

1. Create a directory for the catalog by running the following command:

    ```sh
    mkdir <catalog_dir>
    ```

1. In the same directory level, create a Dockerfile that can build a catalog image:

     ```sh
    touch Dockerfile
    ```

    * The Dockerfile must be in the same parent directory as the catalog directory that you created in the previous step:
        *Example directory structure*

        ```sh
        .
        ├── <catalog_dir>
        └── <catalog_dir>.Dockerfile
        ```

1. Make the following changes to your Dockerfile:
  
    *Example Dockerfile*

    ```sh
        FROM scratch
        ADD <catalog_dir> /configs
    ```

    > **Note:** Use the `FROM scratch` directive to make the size of the image smaller.

1. Populate the catalog with the package definition for your Operator by running the `opm init` command:

    ```sh
    opm init <operator_name> \
    --output json \
    > <catalog_dir>/index.json
    ```

    This command generates an `olm.package` declarative config blob in the specified catalog configuration file.

## Adding a plain bundle to your file-based catalog

Currently, the `opm render` command does not support adding plain bundles to catalogs. You must manually add plain bundles to your file-based catalog, as shown in the following example.

<h3 id="prereqs-adding-a-plain-bundle-to-fbc">
Prerequisites
</h3>

* `opm` CLI tool
* A plain bundle image
* A file-based catalog
* Push access to a container registry, such as [Quay](https://quay.io)
* Docker or Podman

<h3 id="proc-adding-a-plain-bundle-to-fbc">
Procedure
</h3>

1. Verify that your catalog's `index.json` or `index.yaml` file is similar to the following example:

    *Example `<catalog_dir>/index.json` file*

    ```json
    {   
        {
         "schema": "olm.package",
         "name": "<operator_name>",
        }    
    }
    ```

1. To create an `olm.bundle` blob, edit your `index.json` or `index.yaml` file, similar to the following example:

    *Example `<catalog_dir>/index.json` file*

    ```json
    {
        "schema": "olm.bundle",
        "name": "<operator_name>.v<version>",
        "package": "<operator_name>",
        "image": "quay.io/<organization_name>/<repository_name>:<image_tag>", 
        "properties": [
            {
                "type": "olm.package",
                "value": {
                "packageName": "<operator_name>",
                "version": "<bundle_version>"
                }
            },
            {
                "type": "olm.bundle.mediatype",
                "value": "plain+v0"
            }
      ]
    }
    ```

1. To create an `olm.channel` blob, edit your `index.json` or `index.yaml` file, similar to the following example:

    *Example `<catalog_dir>/index.json` file*

    ```json
    {
        "schema": "olm.channel",
        "name": "<desired_channel_name>",
        "package": "<operator_name>",
        "entries": [
            {
                "name": "<operator_name>.v<version>"
            }
        ]
    }
    ```

    > **Note:** Please refer to [channel naming conventions](https://olm.operatorframework.io/docs/best-practices/channel-naming/) for choosing the <desired_channel_name>. An example of the <desired_channel_name> is `candidate-v0`.

<h3 id="verify-adding-a-plain-bundle-to-fbc">
Verification
</h3>

1. Open your `index.json` or `index.yaml` file and ensure it is similar to the following example:

    *Example `index.json` file*

    ```json
    {
        "schema": "olm.package",
        "name": "example-operator",
    }
    {
        "schema": "olm.bundle",
        "name": "example-operator.v0.0.1",
        "package": "example-operator",
        "image": "quay.io/rashmigottipati/example-operator-bundle:v0.0.1",
        "properties": [
            {
                "type": "olm.package",
                "value": {
                "packageName": "example-operator",
                "version": "v0.0.1"
                }
            },
            {
                "type": "olm.bundle.mediatype",
                "value": "plain+v0"
            }
        ]
    }
    {
        "schema": "olm.channel",
        "name": "preview",
        "package": "example-operator",
        "entries": [
            {
                "name": "example-operator.v0.0.1"
            }
        ]
    }
    ```

## Building and publishing a file-based catalog

<h3 id="proc-building-and-publishing-a-fbc">
Procedure
</h3>

1. Run the following command to build your catalog as an image:

    ```sh
    docker build -f <catalog_dir>.Dockerfile -t \
    quay.io/<organization_name>/<repository_name>:<image_tag> .
    ```

1. Run the following command to push the catalog image:

    ```sh
    docker push quay.io/<organization_name>/<repository_name>:<image_tag>
    ```
