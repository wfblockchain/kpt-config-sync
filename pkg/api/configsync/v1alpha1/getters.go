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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kpt.dev/configsync/pkg/api/configsync"
)

// GetPeriodSecs returns the sync period defaulting to 15 if empty.
func GetPeriodSecs(g *Git) float64 {
	if g.Period.Duration == 0 {
		return configsync.DefaultReconcilerPollingPeriodSeconds
	}
	return g.Period.Duration.Seconds()
}

// SafeOverride creates an override or returns an existing one
// use it if you need to ensure that you are assigning
// to an object, but not to test for nil (current existance)
func (rs *RepoSyncSpec) SafeOverride() *OverrideSpec {
	if rs.Override == nil {
		rs.Override = &OverrideSpec{}
	}
	return rs.Override
}

// SafeOverride creates an override or returns an existing one
// use it if you need to ensure that you are assigning
// to an object, but not to test for nil (current existance)
func (rs *RootSyncSpec) SafeOverride() *OverrideSpec {
	if rs.Override == nil {
		rs.Override = &OverrideSpec{}
	}
	return rs.Override
}

// GetReconcileTimeout returns reconcile timeout in string, defaulting to 5m if empty
func GetReconcileTimeout(d *metav1.Duration) string {
	if d == nil || d.Duration == 0 {
		return configsync.DefaultReconcileTimeout.String()
	}
	return d.Duration.String()
}
