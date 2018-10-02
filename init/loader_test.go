package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestLoader(t *testing.T) {
	os.Remove("test.db")
	store, err := NewSQLLiteStore("test.db")
	//defer os.Remove("test.db")
	require.NoError(t, err)

	loader := NewSQLLoaderForDirectory(store, "../manifests")
	require.NoError(t, loader.Populate())


	// what csv does this one replace?
    //	sqlquery := `
    //  SELECT DISTINCT json_extract(operatorbundle.csv, '$.spec.replaces')
    //  FROM operatorbundle,json_tree(operatorbundle.csv)
    //  WHERE operatorbundle.name IS "etcdoperator.v0.9.2"
    //`

    // what replaces this CSV?
	//sqlquery := `
	//SELECT DISTINCT operatorbundle.name
	//FROM operatorbundle,json_tree(operatorbundle.csv, '$.spec.replaces') WHERE json_tree.value = "etcdoperator.v0.9.0"
	//`

	// what apis does this csv provide?
	//sqlquery := `
	//SELECT DISTINCT json_extract(json_each.value, '$.name', '$.version', '$.kind')
	//FROM operatorbundle,json_each(operatorbundle.csv, '$.spec.customresourcedefinitions.owned')
	//WHERE operatorbundle.name IS "etcdoperator.v0.9.2"
	//`

	// what csvs provide this api?
	//sqlquery := `
	//SELECT DISTINCT operatorbundle.name
	//FROM operatorbundle,json_each(operatorbundle.csv, '$.spec.customresourcedefinitions.owned')
	//WHERE json_extract(json_each.value, '$.name') = "etcdclusters.etcd.database.coreos.com"
    //AND  json_extract(json_each.value, '$.version') =  "v1beta2"
    //AND json_extract(json_each.value, '$.kind') = "EtcdCluster"
    //`
	//rows, err := conn.QueryContext(context.TODO(), sqlquery)
	//require.NoError(t, err)
	//
	//for rows.Next() {
	//	var replaces sql.NullString
	//	require.NoError(t, rows.Scan(&replaces))
	//	t.Log(replaces)
	//}
}

