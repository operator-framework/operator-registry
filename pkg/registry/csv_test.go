package registry

import (
	"encoding/json"
	"reflect"
	"testing"

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
				Spec:       json.RawMessage(`
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
					Group: "g",
					Kind: "K",
					Version: "v1",
					Name: "Ks.g",
				},
			},
			wantRequired: []*DefinitionKey{
				{
					Group: "g2",
					Kind: "K2",
					Version: "v1",
					Name: "K2s.g",
				},
			},
		},
		{
			name: "v1alpha1 with owned",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`
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
					Group: "g",
					Kind: "K",
					Version: "v1",
					Name: "Ks.g",
				},
			},
		},
		{
			name: "v1alpha1 with required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`
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
					Group: "g2",
					Kind: "K2",
					Version: "v1",
					Name: "K2s.g",
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
				Spec:       json.RawMessage(`
				{ 
				  "apiservicedefinitions": {
					splat: [
						{"glarp": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),

			},
			wantErr:true,
		},	}
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
				Spec:       json.RawMessage(`
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
					Group: "g",
					Kind: "K",
					Version: "v1",
					Name: "Ks.g",
				},
			},
			wantRequired: []*DefinitionKey{
				{
					Group: "g2",
					Kind: "K2",
					Version: "v1",
					Name: "K2s.g",
				},
			},
		},
		{
			name: "v1alpha1 with owned",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`
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
					Group: "g",
					Kind: "K",
					Version: "v1",
					Name: "Ks.g",
				},
			},
		},
		{
			name: "v1alpha1 with required",
			fields: fields{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{},
				Spec:       json.RawMessage(`
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
					Group: "g2",
					Kind: "K2",
					Version: "v1",
					Name: "K2s.g",
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
				Spec:       json.RawMessage(`
				{ 
				  "customresourcedefinitions": {
					splat: [
						{"glarp": "g2", "version": "v1", "kind": "K2", "name": "K2s.g"}
					] 
				  } 
				}`),

			},
			wantErr:true,
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
		},	}
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
