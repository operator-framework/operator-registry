# Building a file-based catalog from a plain bundle image

**IMPORTANT:** Operator Lifecycle Manager (OLM) v1 features and components are still experimental. Early adopters and contributors should expect breaking changes. The following procedures are not recommended for use on production clusters.

You can build a static collection of arbitrary Kubernetes manifests in the YAML format, or *plain bundle*, and add the image to a file-based catalog (FBC). The experimental `olm.bundle.mediatype` property of the `olm.bundle` schema object differentiates a plain bundle from a regular bundle. You must set the bundle media type property to `plain+v0` to specify a plain bundle.

For more information, see the [Plain Bundle Specification](https://github.com/operator-framework/rukpak/blob/main/docs/bundles/plain.md) in the RukPak repository.

To build a file-based catalog from a plain bundle image, you must complete the following steps:

* Create a plain bundle image
* Create a file-based catalog
* Add the plain bundle image to your file-based catalog
* Build your catalog as an image
* Publish your catalog image

## Building a plain bundle image from an image source

Currently, the Operator Controller only supports building a plain bundle image from an image source.

*Prerequisites*

* [`opm` CLI tool](https://github.com/operator-framework/operator-registry/releases)
* Docker or Podman
* Push access to an image registry, such as [Quay](https://quay.io)

*Procedure*

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

    * If you are using [kustomize](https://kustomize.io) to build your manifests from templates, you must redirect the output into a single file under the `manifests/` directory by running the following command:

        ```sh
        kustomize build templates > manifests/manifests.yaml
        ```

   For more information, see [Building a plain bundle > Prerequisites](https://github.com/operator-framework/rukpak/blob/main/docs/bundles/plain.md#prerequisites).

1. Create a Dockerfile at the root of your project:

    ```sh
    touch Dockerfile.plainbundle
    ```

1. Make the following changes to your Dockerfile:

    *Example Dockerfile*

    ```sh
    FROM scratch
    COPY manifests /manifests
    ```

    **NOTE:** Use the `FROM scratch` directive to make the size of the image smaller.

1. Add the following labels to your Dockerfile:

    ```sh
    LABEL operators.operatorframework.io.bundle.mediatype.v1=plain+v0
    LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
    LABEL operators.operatorframework.io.bundle.package.v1=testOperator
    LABEL operators.operatorframework.io.bundle.channels.v1=preview
    LABEL operators.operatorframework.io.bundle.channel.default.v1=preview
    ```

1. Build an OCI-compliant image using your preferred build tool, similar to the following example. You must use an image tag that references a repository where you have push access privileges.

    *Example build command*

    ```sh
    docker build -f Dockerfile.plainbundle -t \
    quay.io/<image_repo>/plainbundle:example .
    ```

1. Push the image to your remote registry:

    ```sh
    docker push quay.io/<image_repo>/plainbundle:example
    ```

*Additional resources*

* [File-based catalog bundle schema](https://github.com/operator-framework/olm-docs/blob/master/content/en/docs/Reference/file-based-catalogs.md)
* [OCI image specification](https://github.com/opencontainers/image-spec#oci-image-format-specification)
* [RukPak > Building a plain bundle > Image source](https://github.com/operator-framework/rukpak/blob/main/docs/bundles/plain.md#image-source)
* [RukPak > Sources > Images > Private image registries](https://github.com/operator-framework/rukpak/blob/main/docs/sources/image.md#private-image-registries)

## Creating a file-based catalog

If you do not have a file-based catalog, you must perform the following steps to initialize the catalog.

*Prerequisites*

* `opm` CLI tool
* Docker or Podman

*Procedure*

1. Create a directory for the catalog by running the following command:

    ```sh
    mkdir <catalog_dir>
    ```

1. Create a Dockerfile that can build a catalog image by running the `opm generate dockerfile` command:

    *Example `<catalog_name>-image.Dockerfile`*

    ```sh
    opm generate dockerfile <catalog_dir>
    ```

    *TIP:* If do not want to use the default upstream base image, you can specify a different image using the `-i` flag.

    * The Dockerfile must be in the same parent directory as the catalog directory that you created in the previous step:
        *Example directory structure*

        ```sh
        .
        ├── <catalog_dir>
        └── <catalog_dir>.Dockerfile
        ```

1. Populate the catalog with the package definition for your Operator by running the `opm init` command:

    ```sh
    opm init <operator_name> \
    --default-channel=preview \
    --description=./README.md \
    --icon=./operator-icon.svg \
    --output yaml \
    > <catalog_dir>/index.yaml
    ```

    This command generates an `olm.package` declarative config blob in the specified catalog configuration file.

## Adding a plain bundle to your file-based catalog

Currently, the `opm render` command does not support adding plain bundles to catalogs. You must manually add plain bundles to your file-based catalog, as shown in the example below.

*Prerequisites*

* `opm` CLI tool
* A plain bundle image
* A file-based catalog
* Push access to an image registry, such as [Quay](https://quay.io)
* Docker or Podman

*Procedure*

1. Verify that your catalog's `index.json` or `index.yaml` file is similar to the following example:

    *Example `<catalog_dir>/index.json` file*

    ```json
    {   
        {
         "schema": "olm.package",
         "name": "<operator_name>",
         "defaultChannel": "preview"
        }    
    }
    ```

1. To create an `olm.bundle` blob, edit your `index.json` or `index.yaml` file, similar to the following example:

    *Example `<catalog_dir>/index.json` file*

    ```json
    {
        "schema": "olm.bundle",
        "name": "plainbundle:test",
        "package": "<operator_name>",
        "image": "quay.io/<image_repo>/plainbundle:test",
        "properties": [
            {
                "type": "olm.package",
                "value": {
                "packageName": "<operator_name>",
                "version": "0.0.3",
                "olm.bundle.mediatype": "plain+v0"
                }
            }
      ]
    }
    ```

1. To create an `olm.channel` blob, edit your `index.json` or `index.yaml` file, similar to the following example:

    *Example `<catalog_dir>/index.json` file*

    ```json
    {
        "schema": "olm.channel",
        "name": "preview",
        "package": "<operator_name>",
        "entries": [
            {
                "name": "plainbundle:test"
            }
        ]
    }
    ```

*Verification*

* Run the following command to validate your catalog:

    ```sh
    opm validate <catalog_directory>
    ```

    If there are no errors in your catalog, the command completes without output.

## Building and publishing a file-based catalog

*Procedure*

1. Run the following command to build your catalog as an image:

    ```sh
    docker build -f <Dockerfile> -t \
    quay.io/<image_repo>/plain-bundle-catalog:latest
    ```

1. Run the following command to push the catalog image:

    ```sh
    docker push quay.io/<image_repo>/plain-bundle-catalog:latest
    ```
