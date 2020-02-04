package model

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func TestInterfaceField(t *testing.T) {
	dbName := fmt.Sprintf("test-%d.db", rand.Int())
	defer os.Remove(dbName)

	bdb, err := storm.Open(dbName, storm.Codec(Codec))
	require.NoError(t, err)
	defer bdb.Close()

	tx, err := bdb.Begin(true)
	require.NoError(t, err)

	api := Api{
		Group:   "group",
		Version: "v1alpha1",
		Kind:    "Kind",
		Plural:  "kinds",
	}
	capFake := Capability{
		Name:  GvkCapability,
		Value: &api,
	}
	o := OperatorBundle{Name: "Test", Capabilities: []Capability{capFake}}
	require.NoError(t, tx.Save(&o))
	require.NoError(t, tx.Commit())

	var out []OperatorBundle
	require.NoError(t, bdb.All(&out))
	require.Equal(t, 1, len(out))
	require.EqualValues(t, capFake, out[0].Capabilities[0])
}
