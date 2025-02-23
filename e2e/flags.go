// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package e2e defines e2e-test-specific imports and flags for use in e2e
// testing.
package e2e

import (
	"flag"
	"fmt"
	"testing"

	// kubectl auth provider plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// E2E enables running end-to-end tests.
var E2E = flag.Bool("e2e", false,
	"If true, run end-to-end tests.")

// Load enables running of load tests.
var Load = flag.Bool("load", false,
	"If true, run load tests.")

// Stress enables running of stress tests.
var Stress = flag.Bool("stress", false,
	"If true, run stress tests.")

// Kcc enables running the e2e tests for kcc resources.
var Kcc = flag.Bool("kcc", false,
	"If true, run kcc tests.")

// GceNode enables running the e2e tests for 'gcenode' auth type
var GceNode = flag.Bool("gcenode", false,
	"If true, run test with 'gcenode' auth type.")

// Debug enables running the test in debug mode.
// In debug mode:
//  1. Test execution immediately stops on a call to t.Fatal.
//  2. The test prints the absolute path to the test temporary directory, and
//     not delete it.
//  3. The test prints out how to connect to the kind cluster.
var Debug = flag.Bool("debug", false,
	"If true, do not destroy cluster and clean up temporary directory after test.")

// KubernetesVersion is the version of Kubernetes to test against. Only has effect
// when testing against test-created Kind clusters.
var KubernetesVersion = flag.String("kubernetes-version", "1.23",
	"The version of Kubernetes to create")

// DefaultImagePrefix points to the local docker registry.
const DefaultImagePrefix = "localhost:5000"

// Manual indicates the test is being run manually. Some tests are not yet safe
// to be run automatically.
var Manual = flag.Bool("manual", false,
	"Specify that the test is being run manually.")

// TestCluster specifies the cluster config used for testing.
var TestCluster = flag.String("test-cluster", Kind,
	fmt.Sprintf("The cluster config used for testing. Allowed values are: %s and %s. "+
		"If --test-cluster=%s, create a Kind cluster. Otherwise use the GKE context specified in %s.",
		GKE, Kind, Kind, Kubeconfig))

// ShareTestEnv indicates whether to share the test env for all test cases.
// If it is true, we only install nomos once before all tests and tear it down until all tests complete.
var ShareTestEnv = flag.Bool("share-test-env", false,
	"Specify that the test is using a shared test environment instead of fresh installation per test case.")

// GitProvider is the provider that hosts the Git repositories.
var GitProvider = flag.String("git-provider", Local,
	"The git provider that hosts the Git repositories. Defaults to local.")

// TestFeatures is the list of features to run.
var TestFeatures = flag.String("test-features", "",
	"A list of features to run, separated by comma. Defaults to empty, which should run all tests.")

const (
	// Kind indicates creating a Kind cluster for testing.
	Kind = "kind"
	// GKE indicates using an existing GKE cluster for testing.
	GKE = "gke"
	// Kubeconfig provides the context via KUBECONFIG for testing.
	Kubeconfig = "kube-config"
)

const (
	// Local indicates using a local git-test-server.
	Local = "local"
	// Bitbucket indicates using Bitbucket to host the repositories.
	Bitbucket = "bitbucket"
	// Github indicates using GitHub to host the repositories.
	Github = "github"
	// GitLab indicates using GitLab to host the repositories.
	GitLab = "gitlab"
	// CSR indicates using Google Cloud Source Repositories to host the repositories.
	CSR = "csr"
)

// NumParallel returns the number of parallel test threads
func NumParallel() int {
	return flag.Lookup("test.parallel").Value.(flag.Getter).Get().(int)
}

// RunInParallel indicates whether the test is running in parallel.
func RunInParallel() bool {
	return NumParallel() > 1
}

// EnableParallel allows parallel execution of test functions that call t.Parallel
// if test.parallel is greater than 1.
func EnableParallel(t *testing.T) {
	if RunInParallel() {
		t.Parallel()
	}
}
