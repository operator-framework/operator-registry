package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

func newUnstructuredCSVWithSubstitutesFor(t *testing.T, name, substitutesFor string) *unstructured.Unstructured {
	csv := &registry.ClusterServiceVersion{}
	csv.Kind = "ClusterServiceVersion"
	csv.SetName(name)
	csv.SetAnnotations(map[string]string{"olm.substitutesFor": substitutesFor})
	csv.Spec = json.RawMessage(fmt.Sprintf(`{"version": %q}`, "1.0.0"))

	out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(csv)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: out}
}

func newSubstitutesForLoader(t *testing.T) (*sqlLoader, func()) {
	db, cleanup := CreateTestDB(t)
	store, err := NewSQLLiteLoader(db, WithEnableAlpha(true))
	require.NoError(t, err)
	require.NoError(t, store.Migrate(context.TODO()))
	return store.(*sqlLoader), cleanup
}

// Reproducer for OCPBUGS-37284: a bundle whose olm.substitutesFor annotation
// references its own CSV name sends addSubstitutesFor into an unbounded chain
// walk that grows the skips slice until the process runs out of memory.
// It must be rejected with an error instead.
func TestSubstitutesForSelfReference(t *testing.T) {
	loader, cleanup := newSubstitutesForLoader(t)
	defer cleanup()

	self := newBundle(t, "csv-self", "pkg", []string{"stable"},
		newUnstructuredCSVWithSubstitutesFor(t, "csv-self", "csv-self"))

	err := loader.AddOperatorBundle(self)
	require.ErrorContains(t, err, "cyclic substitutesFor")
}

// An indirect cycle (A substitutes for B, B substitutes for A) must also be
// rejected instead of looping forever.
func TestSubstitutesForIndirectCycle(t *testing.T) {
	loader, cleanup := newSubstitutesForLoader(t)
	defer cleanup()

	a := newBundle(t, "csv-a", "pkg", []string{"stable"},
		newUnstructuredCSVWithSubstitutesFor(t, "csv-a", "csv-b"))
	// csv-b does not exist yet, so adding csv-a is legal
	require.NoError(t, loader.AddOperatorBundle(a))

	b := newBundle(t, "csv-b", "pkg", []string{"stable"},
		newUnstructuredCSVWithSubstitutesFor(t, "csv-b", "csv-a"))

	err := loader.AddOperatorBundle(b)
	require.ErrorContains(t, err, "cyclic substitutesFor")
}
