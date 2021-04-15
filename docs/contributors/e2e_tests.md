# Execute end-to-end tests for local development

To execute the end-to-end (e2e) test suite on your local development system, you have two options:

1. Use a dynamically generated kind server. Each run of the test suite will automatically start a new kind cluster,
and will be torn down upon completion of tests.

1. Stand up your own minikube or other cluster and use kubeconfig. This option is good for starting a cluster and keeping it
running even after the test suite has completed.

## Kind without SSL

1. Start a registry server without using SSL

   ```bash
   ./scripts/start_registry.sh -s kind-registry
   ```

1. Start the e2e tests:

   ```bash
   DOCKER_REGISTRY_HOST=localhost:5000 make build e2e SKIPTLS="true" CLUSTER=kind
   ```

1. Run a specific BDD test using the `TEST` argument to make. Note that this argument uses regular expressions.

   ```bash
   DOCKER_REGISTRY_HOST=localhost:5000 make build e2e TEST='builds and manipulates bundle and index images' SKIPTLS="true" CLUSTER=kind
   ```

1. If you want a quick way to ensure that your TEST regex argument will work, you can bypass the 
make file and use `-dryRun` with `-focus` and see if the regex would trigger your specific test(s).

   ```bash
   GOFLAGS="-mod=vendor" go run github.com/onsi/ginkgo/ginkgo --v --randomizeAllSpecs --randomizeSuites --race -dryRun -focus 'builds and manipulates bundle and index images' -tags=json1,kind ./test/e2e
   ```

## Kind with SSL

1. Start a registry server with SSL (NOTE: SSL CA root will be installed and server cert/key will be generated)

   ```bash
   ./scripts/start_registry.sh kind-registry
   ```

1. Start the e2e tests:

   ```bash
   DOCKER_REGISTRY_HOST=localhost:443 make build e2e CLUSTER=kind
   ```

1. Run a specific BDD test using the `TEST` argument to make. Note that this argument uses regular expressions.

   ```bash
   DOCKER_REGISTRY_HOST=localhost:443 make build e2e TEST='builds and manipulates bundle and index images' CLUSTER=kind
   ```

1. If you want a quick way to ensure that your TEST regex argument will work, you can bypass the 
make file and use `-dryRun` with `-focus` and see if the regex would trigger your specific test(s).

   ```bash
   GOFLAGS="-mod=vendor" go run github.com/onsi/ginkgo/ginkgo --v --randomizeAllSpecs --randomizeSuites --race -dryRun -focus 'builds and manipulates bundle and index images' -tags=json1,kind ./test/e2e
   ```

## Minikube (or other type) using kubeconfig without SSL

1. Install `minikube` (see https://minikube.sigs.k8s.io/docs/start/)

1. Create a minikube cluster

   ```bash
   minikube start
   ```

1. Start a registry server without using SSL

   ```bash
   ./scripts/start_registry.sh -s minikube-registry
   ```

1. Start the e2e tests:

   ```bash
   KUBECONFIG="$HOME/.kube/config" DOCKER_REGISTRY_HOST=localhost:5000 make build e2e SKIPTLS="true"
   ```

1. Run a specific BDD test using the `TEST` argument to make. Note that this argument uses regular expressions.

   ```bash
   KUBECONFIG="$HOME/.kube/config" DOCKER_REGISTRY_HOST=localhost:5000 make build e2e TEST='builds and manipulates bundle and index images' SKIPTLS="true"
   ```

1. If you want a quick way to ensure that your TEST regex argument will work, you can bypass the 
make file and use `-dryRun` with `-focus` and see if the regex would trigger your specific test(s).

   ```bash
   GOFLAGS="-mod=vendor" go run github.com/onsi/ginkgo/ginkgo --v --randomizeAllSpecs --randomizeSuites --race -dryRun -focus 'builds and manipulates bundle and index images' -tags=json1 ./test/e2e
   ```

TIP: use a non-dynamic `kind` server by using `kind get kubeconfig --name "kind" > /tmp/kindconfig` and set `KUBECONFIG="/tmp/kindconfig"`

## Minikube (or other type) using kubeconfig with SSL

1. Install `minikube` (see https://minikube.sigs.k8s.io/docs/start/)

1. Create a minikube cluster

   ```bash
   minikube start
   ```

1. Start a registry server with SSL (NOTE: SSL CA root will be installed and server cert/key will be generated)

   ```bash
   ./scripts/start_registry.sh minikube-registry
   ```

1. Start the e2e tests:

   ```bash
   KUBECONFIG="$HOME/.kube/config" DOCKER_REGISTRY_HOST=localhost:443 make build e2e
   ```

1. Run a specific BDD test using the `TEST` argument to make. Note that this argument uses regular expressions.

   ```bash
   KUBECONFIG="$HOME/.kube/config" DOCKER_REGISTRY_HOST=localhost:443 make build e2e TEST='builds and manipulates bundle and index images'
   ```

1. If you want a quick way to ensure that your TEST regex argument will work, you can bypass the 
make file and use `-dryRun` with `-focus` and see if the regex would trigger your specific test(s).

   ```bash
   GOFLAGS="-mod=vendor" go run github.com/onsi/ginkgo/ginkgo --v --randomizeAllSpecs --randomizeSuites --race -dryRun -focus 'builds and manipulates bundle and index images' -tags=json1 ./test/e2e
   ```

TIP: use a non-dynamic `kind` server by using `kind get kubeconfig --name "kind" > /tmp/kindconfig` and set `KUBECONFIG="/tmp/kindconfig"`

## Known Limitations

Currently the test case `Launch bundle` in test/e2e/bundle_image_test.go assumes that the `opm` executable used in the test is compiled for linux.
If you run this test on a darwin environment, the kube job will not succeed unless you manually cross compile for linux and include
the binary at `bin/opm`. 