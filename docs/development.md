# Development
This doc describes how to set up your development environment for Config Sync.

## Requirements
You must have the following tools:
* [go]
* [git]
* [make]
* [docker]

## Checkout the code
The first step is to check out the code for Config Sync to your local
development environment. We recommend that you [create your own fork], but we will
keep things simple here.

```
git clone git@github.com:GoogleContainerTools/kpt-config-sync.git
cd kpt-config-sync
```

## Run tests

### Unit tests
Unit tests are small focused test that runs quickly. Run them with:
```
make test
```

### E2E tests

Config Sync also has e2e tests. These can be run on [kind] or GKE and can take
a long time to finish.

The e2e tests will use the most recently built manifests on your local filesystem,
which are written to the `.output/staging` directory. These are created
when running `make` targets such as `build-manifests` and `config-sync-manifest`.
See [building from source](#build-from-source) for more information.

For the complete list of arguments accepted by the e2e tests, see [flags.go](../e2e/flags.go).
Below is a non-exhaustive list of some useful arguments for running the e2e tests.
These can be provided on the command line with `go test` or with program arguments in your IDE.

- `--e2e` - If true, run end-to-end tests. (required to run the e2e tests)
- `--debug` - If true, do not destroy cluster and clean up temporary directory after test.
- `--share-test-env` - Specify that the test is using a shared test environment instead of fresh installation per test case.
- `--test-cluster` - The cluster config used for testing. Allowed values are: `kind` and `gke`.

Here are some useful flags from [go test](https://pkg.go.dev/cmd/go#hdr-Testing_flags):
- `--test.v` - More verbose output
- `--test.parallel` -- Allow parallel execution of tests (only available for [kind])
- `--test.run` -- Run only tests matching the provided regular expression

### E2E tests (kind)

This section provides instructions on how to run the e2e tests on [kind].

#### Prerequisites

Install [kind]
```shell
# install the kind version specified in https://github.com/GoogleContainerTools/kpt-config-sync/blob/main/scripts/docker-registry.sh#L50
go install sigs.k8s.io/kind@v0.14.0
```

- This will put kind in `$(go env GOPATH)/bin`. This directory will need to be added to `$PATH` if it isn't already.
- After upgrading kind, you usually need to delete all your existing kind clusters so that kind functions correctly.
- Deleting all running kind clusters usually fixes kind issues.

#### Running E2E tests on kind

Run all of the tests (this will take a while):
```
make test-e2e-go-multirepo
```

To execute e2e multi-repo tests locally with kind, build and push the Config Sync
images to the local kind registry and then execute tests using go test.
```shell
make config-sync-manifest-local
go test ./e2e/... --e2e --debug --test.v --test.run (test name regexp)
```

To use already existing images without building everything from scratch, rebuild
only the manifests and then rerun the tests.
```shell
make build-manifests IMAGE_TAG=<tag>
go test ./e2e/... --e2e <additional-options>
```

### E2E tests (GKE)

This section provides instructions on how to run the e2e tests on GKE.

#### Prerequisites

Follow the [instructions to provision a dev environment].

#### Running E2E tests on GKE

To execute e2e multi-repo tests with a GKE cluster, build and push the Config Sync
images to GCR and then use go test. The images will be pushed to the GCP project
from you current gcloud context and the tests will execute on the cluster set as the
current context in your kubeconfig.
```shell
# Ensure gcloud context is set to correct project
gcloud config set project <PROJECT_ID>
# Build images/manifests and push images
make config-sync-manifest
# Set env vars for GKE cluster
export GCP_PROJECT=<PROJECT_ID>
export GCP_CLUSTER=<CLUSTER_NAME>
# One of GCP_REGION and GCP_ZONE must be set (but not both)
export GCP_REGION=<REGION>
export GCP_ZONE=<ZONE>
# Run the tests with image prefix/tag from previous step and desired test regex
go test ./e2e/... --e2e --debug --test.v --share-test-env=true --test.parallel=1 --test-cluster=gke --test.run (test name regexp)
```

## Build

The make targets use default values for certain variables which can be
overridden at runtime. For the full list of variables observed by the make
targets, see [Makefile](../Makefile). Below is a non-exhaustive list of some
useful variables observed by the make targets.

- `REGISTRY` - Registry to use for image tags. Defaults to `gcr.io/<gcloud-context>`.
- `IMAGE_TAG` - Version to use for image tags. Defaults to `git describe`.

> **_Note:_**
The full image tags are constructed using `$(REGISTRY)/<image-name>:$(IMAGE_TAG)`.

Here is an example for how these can be provided at runtime:
```shell
make build-images IMAGE_TAG=latest
```

### Build from source

Config Sync can be built from source with a single command:

```
make config-sync-manifest
```

This will build all the docker images needed for Config Sync and generate
the manifests needed to run it. The images will by default be uploaded to 
Google Container Registry under your current gcloud project and the manifests
will be created in `.output/staging/oss` under the Config Sync directory.

### Subcomponents
Individual components of Config Sync can be built/used with the following
commands. By default images will be tagged for the GCR registry in the current
project. This can be overridden by providing the `REGISTRY` variable at runtime.

Build CLI (nomos):
```shell
make build-cli
```
Build Manifests:
```shell
make build-manifests
```
Build Docker images:
```shell
make build-images
```
Push Docker images:
```shell
make push-images
```
Pull Docker images:
```shell
make pull-images
```
Retag Docker images:
```shell
make retag-images \
 OLD_REGISTRY=gcr.io/baz \
 OLD_IMAGE_TAG=foo \
 REGISTRY=gcr.io/bat \
 IMAGE_TAG=bar 
```

## Run
Running Config Sync is as simple as applying the generated manifests to your
cluster (from the Config Sync directory):

```
kubectl apply -f .output/staging/oss
```

The following make target builds Config Sync and installs it into your cluster:

```
make run-oss
```


[go]: https://go.dev/doc/install
[git]: https://docs.github.com/en/get-started/quickstart/set-up-git
[make]: https://www.gnu.org/software/make/
[docker]: https://www.docker.com/get-started
[create your own fork]: https://docs.github.com/en/get-started/quickstart/fork-a-repo
[kind]: https://kind.sigs.k8s.io/
[instructions to provision a dev environment]: ../e2e/testinfra/terraform/README.md
