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

package reconcile

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"kpt.dev/configsync/pkg/api/configsync"
	"kpt.dev/configsync/pkg/core"
	"kpt.dev/configsync/pkg/declared"
	"kpt.dev/configsync/pkg/kinds"
	"kpt.dev/configsync/pkg/metadata"
	"kpt.dev/configsync/pkg/metrics"
	"kpt.dev/configsync/pkg/remediator/queue"
	"kpt.dev/configsync/pkg/status"
	syncerclient "kpt.dev/configsync/pkg/syncer/client"
	"kpt.dev/configsync/pkg/syncer/syncertest"
	syncertestfake "kpt.dev/configsync/pkg/syncer/syncertest/fake"
	testingfake "kpt.dev/configsync/pkg/syncer/syncertest/fake"
	"kpt.dev/configsync/pkg/testing/fake"
	"kpt.dev/configsync/pkg/testing/testmetrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestWorker_ProcessNextObject(t *testing.T) {
	testCases := []struct {
		name      string
		declared  []client.Object
		toProcess []client.Object
		want      []client.Object
	}{
		{
			name: "update actual objects",
			declared: []client.Object{
				fake.ClusterRoleBindingObject(syncertest.ManagementEnabled,
					core.Label("first", "one")),
				fake.ClusterRoleObject(syncertest.ManagementEnabled,
					core.Label("second", "two")),
			},
			toProcess: []client.Object{
				fake.ClusterRoleBindingObject(syncertest.ManagementEnabled),
				fake.ClusterRoleObject(syncertest.ManagementEnabled),
			},
			want: []client.Object{
				// TODO: Figure out why the reconciler is stripping away labels and annotations.
				fake.ClusterRoleBindingObject(syncertest.ManagementEnabled,
					core.UID("1"), core.ResourceVersion("2"), core.Generation(1),
					core.Label("first", "one")),
				fake.ClusterRoleObject(syncertest.ManagementEnabled,
					core.UID("1"), core.ResourceVersion("2"), core.Generation(1),
					core.Label("second", "two")),
			},
		},
		{
			name:     "delete undeclared objects",
			declared: []client.Object{},
			toProcess: []client.Object{
				fake.ClusterRoleBindingObject(syncertest.ManagementEnabled,
					core.Annotation(metadata.ResourceIDKey, "rbac.authorization.k8s.io_clusterrolebinding_default-name")),
				fake.ClusterRoleObject(syncertest.ManagementEnabled,
					core.Annotation(metadata.ResourceIDKey, "rbac.authorization.k8s.io_clusterrole_default-name")),
			},
			want: []client.Object{},
		},
		{
			name: "create missing objects",
			declared: []client.Object{
				fake.ClusterRoleBindingObject(syncertest.ManagementEnabled),
				fake.ClusterRoleObject(syncertest.ManagementEnabled),
			},
			toProcess: []client.Object{
				queue.MarkDeleted(context.Background(), fake.ClusterRoleBindingObject()),
				queue.MarkDeleted(context.Background(), fake.ClusterRoleObject()),
			},
			want: []client.Object{
				fake.ClusterRoleBindingObject(syncertest.ManagementEnabled,
					core.UID("1"), core.ResourceVersion("1"), core.Generation(1),
				),
				fake.ClusterRoleObject(syncertest.ManagementEnabled,
					core.UID("1"), core.ResourceVersion("1"), core.Generation(1),
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q := queue.New("test")
			for _, obj := range tc.toProcess {
				q.Add(obj)
			}

			c := testingfake.NewClient(t, core.Scheme)
			for _, obj := range tc.toProcess {
				if !queue.WasDeleted(context.Background(), obj) {
					if err := c.Create(context.Background(), obj); err != nil {
						t.Fatalf("Failed to create object in fake client: %v", err)
					}
				}
			}

			d := makeDeclared(t, tc.declared...)
			w := NewWorker(declared.RootReconciler, configsync.RootSyncName, c.Applier(), q, d)

			for _, obj := range tc.toProcess {
				if ok := w.processNextObject(context.Background()); !ok {
					t.Errorf("unexpected false result from processNextObject() for object: %v", obj)
				}
			}

			c.Check(t, tc.want...)
		})
	}
}

// TestWorker_Run_Cancelled verifies that worker.Run can be cancelled when the
// queue is empty.
func TestWorker_Run_CancelledWhenEmpty(t *testing.T) {
	q := queue.New("test") // empty queue
	c := testingfake.NewClient(t, core.Scheme)
	d := makeDeclared(t) // no resources declared
	w := NewWorker(declared.RootReconciler, configsync.RootSyncName, c.Applier(), q, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run worker in the background
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		w.Run(ctx)
	}()

	// Let the worker run for a bit and then stop it.
	time.Sleep(1 * time.Second)
	cancel()

	// Wait for worker to exit or timeout
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	select {
	case <-timeout.C:
		// fail
		t.Error("Run() with empty queue did not return when context was cancelled")
	case <-doneCh:
		// pass
		c.Check(t) // no objects expected
	}
}

// TestWorker_Run_CancelledWhenNotEmpty verifies that worker.Run can be
// cancelled when the queue is not empty.
// Use a fake client Update error to prevent the queue from draining.
func TestWorker_Run_CancelledWhenNotEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	existingObjs := []client.Object{
		fake.ClusterRoleBindingObject(syncertest.ManagementEnabled),
		fake.ClusterRoleObject(syncertest.ManagementEnabled),
	}
	declaredObjs := []client.Object{
		fake.ClusterRoleBindingObject(syncertest.ManagementEnabled),
		fake.ClusterRoleObject(syncertest.ManagementEnabled),
	}
	changedObjs := []client.Object{
		queue.MarkDeleted(ctx, fake.ClusterRoleBindingObject()),
		fake.ClusterRoleObject(syncertest.ManagementEnabled,
			core.Label("new", "label")),
	}
	expectedObjs := []client.Object{
		// CRB delete should be reverted
		fake.ClusterRoleBindingObject(syncertest.ManagementEnabled,
			core.UID("1"), core.ResourceVersion("1"), core.Generation(1),
		),
		// Role revert should fail from fake Update error
		fake.ClusterRoleObject(syncertest.ManagementEnabled,
			core.UID("1"), core.ResourceVersion("2"), core.Generation(1),
			core.Label("new", "label"),
		),
	}

	q := queue.New("test")

	c := testingfake.NewClient(t, core.Scheme)
	for _, obj := range existingObjs {
		if err := c.Create(ctx, obj); err != nil {
			t.Fatalf("Failed to create object in fake client: %v", err)
		}
	}

	d := makeDeclared(t, declaredObjs...)
	a := &testingfake.Applier{Client: c}
	w := NewWorker(declared.RootReconciler, configsync.RootSyncName, a, q, d)

	// Run worker in the background
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		w.Run(ctx)
	}()

	// Run the worker for a bit with an empty queue, to make sure it starts up.
	time.Sleep(1 * time.Second)

	// Execute runtime changes
	for _, obj := range changedObjs {
		if deletedObj, ok := obj.(*queue.Deleted); ok {
			if err := c.Delete(ctx, deletedObj.Object); err != nil {
				t.Fatalf("Failed to delete object in fake client: %v", err)
			}
		} else {
			if err := c.Update(ctx, obj); err != nil {
				t.Fatalf("Failed to update object in fake client: %v", err)
			}
		}
	}

	// Configure the Applier to start erroring on Update.
	// This will prevent the reconciler from reverting the ClusterRoleObject
	// change, and prevent the queue from emptying.
	a.UpdateError = fmt.Errorf("fake update error")

	// Simulate watch events to add the objects to the queue
	for _, obj := range changedObjs {
		q.Add(obj)
	}

	// Cancel the worker before all the changes are processed
	cancel()

	// Wait for worker to exit or timeout
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	select {
	case <-timeout.C:
		// fail
		t.Error("Run() with empty queue did not return when context was cancelled")
	case <-doneCh:
		// pass
		c.Check(t, expectedObjs...)
	}
}

func TestWorker_Refresh(t *testing.T) {
	name := "admin"
	namespace := "shipping"
	scheme := runtime.NewScheme()
	err := v1.AddToScheme(scheme)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name        string
		queue       fakeQueue
		client      client.Client
		want        *unstructured.Unstructured
		wantDeleted bool
		wantErr     status.Error
	}{
		{
			name: "Not found marks object deleted",
			queue: fakeQueue{
				element: fake.UnstructuredObject(kinds.Role(), core.Name(name), core.Namespace(namespace)),
			},
			client:      syncertestfake.NewClient(t, scheme),
			want:        fake.UnstructuredObject(kinds.Role(), core.Name(name), core.Namespace(namespace)),
			wantDeleted: true,
			wantErr:     nil,
		},
		{
			name: "Found updates objects",
			queue: fakeQueue{
				element: fake.UnstructuredObject(kinds.Role(), core.Name(name), core.Namespace(namespace),
					core.Annotation("foo", "bar")),
			},
			client: syncertestfake.NewClient(t, scheme,
				fake.RoleObject(core.Name(name), core.Namespace(namespace),
					core.Annotation("foo", "qux"))),
			want: fake.UnstructuredObject(kinds.Role(), core.Name(name), core.Namespace(namespace),
				core.UID("1"), core.ResourceVersion("1"), core.Generation(1),
				core.Annotation("foo", "qux")),
			wantDeleted: false,
			wantErr:     nil,
		},
		{
			name: "API Error does not update object",
			queue: fakeQueue{
				element: fake.UnstructuredObject(kinds.Role(), core.Name(name), core.Namespace(namespace)),
			},
			client:      syncertestfake.NewErrorClient(errors.New("some error")),
			want:        fake.UnstructuredObject(kinds.Role(), core.Name(name), core.Namespace(namespace)),
			wantDeleted: false,
			wantErr:     status.APIServerError(errors.New("some error"), ""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := &Worker{
				objectQueue: &tc.queue,
				reconciler: fakeReconciler{
					client: tc.client,
				},
			}

			err := w.refresh(context.Background(), fake.UnstructuredObject(
				kinds.Role(), core.Name(name), core.Namespace(namespace)))
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got refresh = %v, want %v",
					err, tc.wantErr)
			}

			if !tc.wantDeleted && tc.wantErr == nil {
				// These fields are added by unstructured conversions, but we aren't
				// testing this behavior.
				_ = unstructured.SetNestedField(tc.want.Object, nil, "metadata", "creationTimestamp")
				_ = unstructured.SetNestedField(tc.want.Object, nil, "rules")
				unstructured.RemoveNestedField(tc.want.Object, "metadata", "labels")
			}

			var want client.Object = tc.want
			if tc.wantDeleted {
				want = queue.MarkDeleted(context.Background(), want)
			}

			if diff := cmp.Diff(want, tc.queue.element); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestWorker_ResourceConflictMetricValidation(t *testing.T) {
	testCases := []struct {
		name        string
		objects     []client.Object
		wantMetrics []*view.Row
	}{
		{
			name:    "single resource conflict for Role object",
			objects: []client.Object{fake.UnstructuredObject(kinds.Role())},
			wantMetrics: []*view.Row{
				{Data: &view.CountData{Value: 1}, Tags: []tag.Tag{{Key: metrics.KeyType, Value: fake.UnstructuredObject(kinds.Role()).GetKind()}}},
			},
		},
		{
			name:    "multiple resource conflicts for Role object",
			objects: []client.Object{fake.UnstructuredObject(kinds.Role()), fake.UnstructuredObject(kinds.Role())},
			wantMetrics: []*view.Row{
				{Data: &view.CountData{Value: 2}, Tags: []tag.Tag{{Key: metrics.KeyType, Value: fake.UnstructuredObject(kinds.Role()).GetKind()}}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := testmetrics.RegisterMetrics(metrics.ResourceConflictsView)

			for _, obj := range tc.objects {
				w := &Worker{
					objectQueue: &fakeQueue{},
					reconciler: fakeReconciler{
						client:       testingfake.NewClient(t, core.Scheme),
						remediateErr: syncerclient.ConflictUpdateDoesNotExist(errors.New("resource conflict error"), obj),
					},
				}
				w.process(context.Background(), obj)
			}
			if diff := m.ValidateMetrics(metrics.ResourceConflictsView, tc.wantMetrics); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

type fakeReconciler struct {
	client       client.Client
	remediateErr status.Error
}

var _ reconcilerInterface = fakeReconciler{}

func (f fakeReconciler) Remediate(_ context.Context, _ core.ID, _ client.Object) status.Error {
	return f.remediateErr
}

func (f fakeReconciler) GetClient() client.Client {
	return f.client
}

type fakeQueue struct {
	queue.Interface
	element client.Object
}

func (q *fakeQueue) Add(o client.Object) {
	q.element = o
}

func (q *fakeQueue) Retry(o client.Object) {
	q.element = o
}

func (q *fakeQueue) Forget(_ client.Object) {
	q.element = nil
}
