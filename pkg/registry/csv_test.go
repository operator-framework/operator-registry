package registry

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterServiceVersion_GetApiServiceDefinitions(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       json.RawMessage
	}
	tests := []struct {
		name         string
		fields       fields
		wantOwned    []*DefinitionKey
		wantRequired []*DefinitionKey
		wantErr      bool
	}{
		{
			name: "v1alpha1 with owned, required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "apiservicedefinitions": {
					"owned": [ 
						{"group": "g", "version": "v1", "kind": "K", "name": "Ks.g"} 
					], 
					"required": [
						{"group": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),
			},
			wantOwned: []*DefinitionKey{
				{
					Group:   "g",
					Kind:    "K",
					Version: "v1",
					Name:    "Ks.g",
				},
			},
			wantRequired: []*DefinitionKey{
				{
					Group:   "g2",
					Kind:    "K2",
					Version: "v1",
					Name:    "K2s.g",
				},
			},
		},
		{
			name: "v1alpha1 with owned",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "apiservicedefinitions": {
					"owned": [ 
						{"group": "g", "version": "v1", "kind": "K", "name": "Ks.g"} 
					]
				  } 
				}`),
			},
			wantOwned: []*DefinitionKey{
				{
					Group:   "g",
					Kind:    "K",
					Version: "v1",
					Name:    "Ks.g",
				},
			},
		},
		{
			name: "v1alpha1 with required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "apiservicedefinitions": {
					"required": [
						{"group": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),
			},
			wantRequired: []*DefinitionKey{
				{
					Group:   "g2",
					Kind:    "K2",
					Version: "v1",
					Name:    "K2s.g",
				},
			},
		},
		{
			name: "v1alpha1 missing owned,required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"replaces": 5}`),
			},
		},
		{
			name: "v1alpha1 malformed owned,required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "apiservicedefinitions": {
					splat: [
						{"glarp": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),
			},
			wantErr: true,
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := &ClusterServiceVersion{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			gotOwned, gotRequired, err := csv.GetApiServiceDefinitions()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetApiServiceDefinitions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotOwned, tt.wantOwned) {
				t.Errorf("GetApiServiceDefinitions() gotOwned = %v, want %v", gotOwned, tt.wantOwned)
			}
			if !reflect.DeepEqual(gotRequired, tt.wantRequired) {
				t.Errorf("GetApiServiceDefinitions() gotRequired = %v, want %v", gotRequired, tt.wantRequired)
			}
		})
	}
}

func TestClusterServiceVersion_GetCustomResourceDefintions(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       json.RawMessage
	}
	tests := []struct {
		name         string
		fields       fields
		wantOwned    []*DefinitionKey
		wantRequired []*DefinitionKey
		wantErr      bool
	}{
		{
			name: "v1alpha1 with owned, required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "customresourcedefinitions": {
					"owned": [ 
						{"group": "g", "version": "v1", "kind": "K", "name": "Ks.g"} 
					], 
					"required": [
						{"group": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),
			},
			wantOwned: []*DefinitionKey{
				{
					Group:   "g",
					Kind:    "K",
					Version: "v1",
					Name:    "Ks.g",
				},
			},
			wantRequired: []*DefinitionKey{
				{
					Group:   "g2",
					Kind:    "K2",
					Version: "v1",
					Name:    "K2s.g",
				},
			},
		},
		{
			name: "v1alpha1 with owned",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "customresourcedefinitions": {
					"owned": [ 
						{"group": "g", "version": "v1", "kind": "K", "name": "Ks.g"} 
					]
				  } 
				}`),
			},
			wantOwned: []*DefinitionKey{
				{
					Group:   "g",
					Kind:    "K",
					Version: "v1",
					Name:    "Ks.g",
				},
			},
		},
		{
			name: "v1alpha1 with required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "customresourcedefinitions": {
					"required": [
						{"group": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),
			},
			wantRequired: []*DefinitionKey{
				{
					Group:   "g2",
					Kind:    "K2",
					Version: "v1",
					Name:    "K2s.g",
				},
			},
		},
		{
			name: "v1alpha1 missing owned,required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"replaces": 5}`),
			},
		},
		{
			name: "v1alpha1 malformed owned,required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{ 
				  "customresourcedefinitions": {
					splat: [
						{"glarp": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := &ClusterServiceVersion{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			gotOwned, gotRequired, err := csv.GetCustomResourceDefintions()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCustomResourceDefintions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotOwned, tt.wantOwned) {
				t.Errorf("GetCustomResourceDefintions() gotOwned = %v, want %v", gotOwned, tt.wantOwned)
			}
			if !reflect.DeepEqual(gotRequired, tt.wantRequired) {
				t.Errorf("GetCustomResourceDefintions() gotRequired = %v, want %v", gotRequired, tt.wantRequired)
			}
		})
	}
}

func TestClusterServiceVersion_GetReplaces(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       json.RawMessage
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "v1alpha1 with replaces",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"replaces": "etcd-operator.v0.9.2"}`),
			},
			want: "etcd-operator.v0.9.2",
		},
		{
			name: "v1alpha1 no replaces",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"other": "field"}`),
			},
			want: "",
		},
		{
			name: "v1alpha1 malformed replaces",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"replaces": 5}`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := &ClusterServiceVersion{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			got, err := csv.GetReplaces()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReplaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetReplaces() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterServiceVersion_GetSkips(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       json.RawMessage
	}
	tests := []struct {
		name    string
		fields  fields
		want    []string
		wantErr bool
	}{
		{
			name: "v1alpha1 with skips",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"skips": ["1.0.5", "1.0.4"]}`),
			},
			want: []string{"1.0.5", "1.0.4"},
		},
		{
			name: "v1alpha1 no skips",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"other": "field"}`),
			},
			want: nil,
		},
		{
			name: "v1alpha1 malformed skips",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"skips": 5}`),
			},
			wantErr: true,
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := &ClusterServiceVersion{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			got, err := csv.GetSkips()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSkips() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSkips() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterServiceVersion_GetVersion(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       json.RawMessage
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "v1alpha1 with version",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"version": "1.0.5"}`),
			},
			want: "1.0.5",
		},
		{
			name: "v1alpha1 no version",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"other": "field"}`),
			},
			want: "",
		},
		{
			name: "v1alpha1 malformed version",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"version": 5}`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := &ClusterServiceVersion{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			got, err := csv.GetVersion()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterServiceVersion_GetRelatedImages(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       json.RawMessage
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[string]struct{}
		wantErr bool
	}{
		{
			name: "no related images",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`{"no": "field"}`),
			},
			want: map[string]struct{}{},
		},
		{
			name: "one related image",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`{"relatedImages": [
					{"name": "test", "image": "quay.io/etcd/etcd-operator@sha256:123"}
				]}`),
			},
			want: map[string]struct{}{"quay.io/etcd/etcd-operator@sha256:123": {}},
		},
		{
			name: "multiple related images",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`{"relatedImages": [
					{"name": "test", "image": "quay.io/etcd/etcd-operator@sha256:123"},
					{"name": "operand", "image": "quay.io/etcd/etcd@sha256:123"}
				]}`),
			},
			want: map[string]struct{}{"quay.io/etcd/etcd-operator@sha256:123": {}, "quay.io/etcd/etcd@sha256:123": {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := &ClusterServiceVersion{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			got, err := csv.GetRelatedImages()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRelatedImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClusterServiceVersion_GetOperatorImages(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       json.RawMessage
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[string]struct{}
		wantErr bool
	}{
		{
			name: "bad strategy",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{"install": {"strategy": "nope", "spec": {"deployments":[{"name":"etcd-operator","spec":{"template":{"spec":{"containers":[{
					"command":["etcd-operator"],
					"image":"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
					"name":"etcd-operator"
				}]}}}}]}}}`),
			},
		},
		{
			name: "no images",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{"install": {"strategy": "deployment","spec": {"deployments":[{"name":"etcd-operator","spec":{"template":{"spec":
					"containers":[]
				}}}}]}}}`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "one image",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{"install": {"strategy": "deployment", "spec": {"deployments":[{
					"name":"etcd-operator",
					"spec":{
						"template":{
							"spec":{
								"containers":[
									{
										"command":["etcd-operator"],
										"image":"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
										"name":"etcd-operator"
									}	
								]
							}
						}
				}}]}}}`),
			},
			want: map[string]struct{}{"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}},
		},
		{
			name: "two container images",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{"install": {"strategy": "deployment", "spec": {"deployments":[{
					"name":"etcd-operator",
					"spec":{
						"template":{
							"spec":{
								"containers":[
									{
										"command":["etcd-operator"],
										"image":"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
										"name":"etcd-operator"
									},
									{
										"command":["etcd-operator-2"],
										"image":"quay.io/coreos/etcd-operator-2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
										"name":"etcd-operator-2"
									}	
								]
							}
						}
				}}]}}}`),
			},
			want: map[string]struct{}{"quay.io/coreos/etcd-operator-2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}, "quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}},
		},
		{
			name: "init container image",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{
					"install": {
						"strategy": "deployment",
						"spec": {
							"deployments":[
								{
									"name":"etcd-operator",
									"spec":{
										"template":{
											"spec":{
												"initContainers":[
													{
														"command":["etcd-operator"],
														"image":"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
														"name":"etcd-operator"
													}	
												]
											}
										}
									}
								}
							]
						}
					}
				}`),
			},
			want: map[string]struct{}{"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}},
		},
		{
			name: "two init container images",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{
					"install": {
						"strategy": "deployment",
						"spec": {
							"deployments":[
								{
									"name":"etcd-operator",
									"spec":{
										"template":{
											"spec":{
												"initContainers":[
													{
														"command":["etcd-operator"],
														"image":"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
														"name":"etcd-operator"
													},
													{
														"command":["etcd-operator2"],
														"image":"quay.io/coreos/etcd-operator2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
														"name":"etcd-operator2"
													}	
												]
											}
										}
									}
								}
							]
						}
					}
				}`),
			},
			want: map[string]struct{}{"quay.io/coreos/etcd-operator2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}, "quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}},
		},
		{
			name: "container and init container",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec: json.RawMessage(`
				{
					"install": {
						"strategy": "deployment",
						"spec": {
							"deployments":[
								{
									"name":"etcd-operator",
									"spec":{
										"template":{
											"spec":{
												"initContainers":[
													{
														"command":["init-etcd-operator"],
														"image":"quay.io/coreos/init-etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
														"name":"etcd-operator"
													},
													{
														"command":["init-etcd-operator2"],
														"image":"quay.io/coreos/init-etcd-operator2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
														"name":"etcd-operator2"
													}	
												],
												"containers":[
													{
														"command":["etcd-operator"],
														"image":"quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
														"name":"etcd-operator"
													},
													{
														"command":["etcd-operator2"],
														"image":"quay.io/coreos/etcd-operator2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2",
														"name":"etcd-operator2"
													}	
												]
											}
										}
									}
								}
							]
						}
					}
				}`),
			},
			want: map[string]struct{}{"quay.io/coreos/etcd-operator2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}, "quay.io/coreos/etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": struct{}{}, "quay.io/coreos/init-etcd-operator2@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}, "quay.io/coreos/init-etcd-operator@sha256:c0301e4686c3ed4206e370b42de5a3bd2229b9fb4906cf85f3f30650424abec2": {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := &ClusterServiceVersion{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			got, err := csv.GetOperatorImages()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOperatorImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoadingCsvFromBundleDirectory(t *testing.T) {
	tests := []struct {
		dir       string
		fail      bool
		name      string
		version   string
		replace   string
		skips     []string
		skipRange string
	}{
		{
			dir:     "./testdata/validPackages/etcd/0.6.1",
			fail:    false,
			name:    "etcdoperator.v0.6.1",
			version: "0.6.1",
		},
		{
			dir:     "./testdata/validPackages/etcd/0.9.0",
			fail:    false,
			name:    "etcdoperator.v0.9.0",
			version: "0.9.0",
			replace: "etcdoperator.v0.6.1",
		},
		{
			dir:       "./testdata/validPackages/etcd/0.9.2",
			fail:      false,
			name:      "etcdoperator.v0.9.2",
			skipRange: "< 0.6.0",
			version:   "0.9.2",
			skips:     []string{"etcdoperator.v0.9.1"},
			replace:   "etcdoperator.v0.9.0",
		},
		{
			dir:     "./testdata/validPackages/prometheus/0.14.0",
			fail:    false,
			name:    "prometheusoperator.0.14.0",
			version: "0.14.0",
		},
		{
			dir:  "testdata/invalidPackges/3scale-community-operator/0.3.0",
			fail: true,
		},
		{
			dir:  "testdata/invalidPackges/3scale-community-operator/0.4.0",
			fail: true,
		},
	}

	for _, tt := range tests {
		t.Run("Loading Package Graph from "+tt.dir, func(t *testing.T) {
			csv, err := ReadCSVFromBundleDirectory(tt.dir)
			if tt.fail {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.EqualValues(t, tt.name, csv.GetName())

			csvVersion, err := csv.GetVersion()
			assert.NoError(t, err)
			assert.EqualValues(t, tt.version, csvVersion)

			assert.EqualValues(t, tt.skipRange, csv.GetSkipRange())

			csvReplace, err := csv.GetReplaces()
			assert.NoError(t, err)
			assert.EqualValues(t, tt.replace, csvReplace)

			csvSkips, err := csv.GetSkips()
			assert.NoError(t, err)
			assert.EqualValues(t, tt.skips, csvSkips)
		})
	}
}
