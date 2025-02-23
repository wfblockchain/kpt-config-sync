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

package ntopts

import (
	"k8s.io/client-go/rest"
	"kpt.dev/configsync/e2e"
	"kpt.dev/configsync/e2e/nomostest/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Opt is an option type for ntopts.New.
type Opt func(opt *New)

// Commit represents a commit to be created on a git repository
type Commit struct {
	// Message is the commit message
	Message string
	// Files is a map of file paths to Objects
	Files map[string]client.Object
}

// New is the set of options for instantiating a new NT test.
type New struct {
	// Name is the name of the test. Overrides the one generated from the test
	// name.
	Name string

	// TmpDir is the base temporary directory to use for the test. Overrides the
	// generated directory based on Name and the OS's main temporary directory.
	TmpDir string

	// RESTConfig is the config for creating a Client connection to a K8s cluster.
	RESTConfig *rest.Config

	// WatchConfig is the config for creating a Client connection to a K8s
	// cluster for watches.
	WatchConfig *rest.Config

	// KubeconfigPath is the path to the kubeconfig file
	KubeconfigPath string

	// SkipConfigSyncInstall skips installation/cleanup of Config Sync
	SkipConfigSyncInstall bool

	// SkipAutopilot will skip the test if running on an Autopilot cluster.
	SkipAutopilot bool

	// InitialCommit commit to create before the initial sync
	InitialCommit *Commit

	// TestFeature is the feature that the test verifies
	TestFeature testing.Feature

	Nomos
	MultiRepo
	TestType
}

// RequireManual requires the --manual flag is set. Otherwise it will skip the test.
// This avoids running tests (e.g stress tests) that aren't safe to run against a remote cluster automatically.
func RequireManual(t testing.NTB) Opt {
	if !*e2e.Manual {
		t.Skip("Must pass --manual so this isn't accidentally run against a test cluster automatically.")
	}
	return func(opt *New) {}
}

// SkipAutopilotCluster will skip the test on the autopilot cluster.
func SkipAutopilotCluster(opt *New) {
	opt.SkipAutopilot = true
}

// RequireGKE requires the --test-cluster flag to be `gke` so that the test only runs on GKE clusters.
func RequireGKE(t testing.NTB) Opt {
	if *e2e.TestCluster != e2e.GKE {
		t.Skip("The --test-cluster flag must be set to `gke` to run this test.")
	}
	return func(opt *New) {}
}

// RequireKind requires the --test-cluster flag to be `kind` so that the test only runs on kind clusters.
func RequireKind(t testing.NTB) Opt {
	if *e2e.TestCluster != e2e.Kind {
		t.Skip("The --test-cluster flag must be set to `kind` to run this test.")
	}
	return func(opt *New) {}
}

// WithInitialCommit creates the initialCommit before the first sync
func WithInitialCommit(initialCommit Commit) func(opt *New) {
	return func(opt *New) {
		opt.InitialCommit = &initialCommit
	}
}

// WithRestConfig uses the provided rest.Config
func WithRestConfig(restConfig *rest.Config) Opt {
	return func(opt *New) {
		opt.RESTConfig = restConfig
	}
}

// WithWatchConfig uses the provided rest.Config for watches
func WithWatchConfig(watchConfig *rest.Config) Opt {
	return func(opt *New) {
		opt.WatchConfig = watchConfig
	}
}

// SkipConfigSyncInstall skip installation of Config Sync components in cluster
func SkipConfigSyncInstall(opt *New) {
	opt.SkipConfigSyncInstall = true
}
