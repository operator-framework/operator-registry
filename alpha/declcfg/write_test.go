package declcfg

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	type spec struct {
		name     string
		cfg      DeclarativeConfig
		expected string
	}
	specs := []spec{
		{
			name: "Success",
			cfg:  buildValidDeclarativeConfig(true),
			expected: `{
    "schema": "olm.package",
    "name": "anakin",
    "defaultChannel": "dark",
    "icon": {
        "base64data": "PHN2ZyB2aWV3Qm94PSIwIDAgMTAwIDEwMCI+PGNpcmNsZSBjeD0iMjUiIGN5PSIyNSIgcj0iMjUiLz48L3N2Zz4=",
        "mediatype": "image/svg+xml"
    },
    "description": "anakin operator"
}
{
    "schema": "olm.channel",
    "name": "dark",
    "package": "anakin",
    "entries": [
        {
            "name": "anakin.v0.0.1"
        },
        {
            "name": "anakin.v0.1.0",
            "replaces": "anakin.v0.0.1"
        },
        {
            "name": "anakin.v0.1.1",
            "replaces": "anakin.v0.0.1",
            "skips": [
                "anakin.v0.1.0"
            ]
        }
    ]
}
{
    "schema": "olm.channel",
    "name": "light",
    "package": "anakin",
    "entries": [
        {
            "name": "anakin.v0.0.1"
        },
        {
            "name": "anakin.v0.1.0",
            "replaces": "anakin.v0.0.1"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "anakin.v0.0.1",
    "package": "anakin",
    "image": "anakin-bundle:v0.0.1",
    "properties": [
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "ref": "objects/anakin.v0.0.1.csv.yaml"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "anakin",
                "version": "0.0.1"
            }
        }
    ],
    "relatedImages": [
        {
            "name": "bundle",
            "image": "anakin-bundle:v0.0.1"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "anakin.v0.1.0",
    "package": "anakin",
    "image": "anakin-bundle:v0.1.0",
    "properties": [
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "ref": "objects/anakin.v0.1.0.csv.yaml"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "anakin",
                "version": "0.1.0"
            }
        }
    ],
    "relatedImages": [
        {
            "name": "bundle",
            "image": "anakin-bundle:v0.1.0"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "anakin.v0.1.1",
    "package": "anakin",
    "image": "anakin-bundle:v0.1.1",
    "properties": [
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "ref": "objects/anakin.v0.1.1.csv.yaml"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "anakin",
                "version": "0.1.1"
            }
        }
    ],
    "relatedImages": [
        {
            "name": "bundle",
            "image": "anakin-bundle:v0.1.1"
        }
    ]
}
{
    "myField": "foobar",
    "package": "anakin",
    "schema": "custom.3"
}
{
    "schema": "olm.package",
    "name": "boba-fett",
    "defaultChannel": "mando",
    "icon": {
        "base64data": "PHN2ZyB2aWV3Qm94PSIwIDAgMTAwIDEwMCI+PGNpcmNsZSBjeD0iNTAiIGN5PSI1MCIgcj0iNTAiLz48L3N2Zz4=",
        "mediatype": "image/svg+xml"
    },
    "description": "boba-fett operator"
}
{
    "schema": "olm.channel",
    "name": "mando",
    "package": "boba-fett",
    "entries": [
        {
            "name": "boba-fett.v1.0.0"
        },
        {
            "name": "boba-fett.v2.0.0",
            "replaces": "boba-fett.v1.0.0"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "boba-fett.v1.0.0",
    "package": "boba-fett",
    "image": "boba-fett-bundle:v1.0.0",
    "properties": [
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "ref": "objects/boba-fett.v1.0.0.csv.yaml"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "boba-fett",
                "version": "1.0.0"
            }
        }
    ],
    "relatedImages": [
        {
            "name": "bundle",
            "image": "boba-fett-bundle:v1.0.0"
        }
    ]
}
{
    "schema": "olm.bundle",
    "name": "boba-fett.v2.0.0",
    "package": "boba-fett",
    "image": "boba-fett-bundle:v2.0.0",
    "properties": [
        {
            "type": "olm.bundle.object",
            "value": {
                "data": "eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0="
            }
        },
        {
            "type": "olm.bundle.object",
            "value": {
                "ref": "objects/boba-fett.v2.0.0.csv.yaml"
            }
        },
        {
            "type": "olm.package",
            "value": {
                "packageName": "boba-fett",
                "version": "2.0.0"
            }
        }
    ],
    "relatedImages": [
        {
            "name": "bundle",
            "image": "boba-fett-bundle:v2.0.0"
        }
    ]
}
{
    "myField": "foobar",
    "package": "boba-fett",
    "schema": "custom.3"
}
{
    "schema": "custom.1"
}
{
    "schema": "custom.2"
}
`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteJSON(s.cfg, &buf)
			require.NoError(t, err)
			require.Equal(t, s.expected, buf.String())
		})
	}
}

func TestWriteYAML(t *testing.T) {
	type spec struct {
		name     string
		cfg      DeclarativeConfig
		expected string
	}
	specs := []spec{
		{
			name: "Success",
			cfg:  buildValidDeclarativeConfig(true),
			expected: `---
defaultChannel: dark
description: anakin operator
icon:
  base64data: PHN2ZyB2aWV3Qm94PSIwIDAgMTAwIDEwMCI+PGNpcmNsZSBjeD0iMjUiIGN5PSIyNSIgcj0iMjUiLz48L3N2Zz4=
  mediatype: image/svg+xml
name: anakin
schema: olm.package
---
entries:
- name: anakin.v0.0.1
- name: anakin.v0.1.0
  replaces: anakin.v0.0.1
- name: anakin.v0.1.1
  replaces: anakin.v0.0.1
  skips:
  - anakin.v0.1.0
name: dark
package: anakin
schema: olm.channel
---
entries:
- name: anakin.v0.0.1
- name: anakin.v0.1.0
  replaces: anakin.v0.0.1
name: light
package: anakin
schema: olm.channel
---
image: anakin-bundle:v0.0.1
name: anakin.v0.0.1
package: anakin
properties:
- type: olm.bundle.object
  value:
    data: eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0=
- type: olm.bundle.object
  value:
    ref: objects/anakin.v0.0.1.csv.yaml
- type: olm.package
  value:
    packageName: anakin
    version: 0.0.1
relatedImages:
- image: anakin-bundle:v0.0.1
  name: bundle
schema: olm.bundle
---
image: anakin-bundle:v0.1.0
name: anakin.v0.1.0
package: anakin
properties:
- type: olm.bundle.object
  value:
    data: eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0=
- type: olm.bundle.object
  value:
    ref: objects/anakin.v0.1.0.csv.yaml
- type: olm.package
  value:
    packageName: anakin
    version: 0.1.0
relatedImages:
- image: anakin-bundle:v0.1.0
  name: bundle
schema: olm.bundle
---
image: anakin-bundle:v0.1.1
name: anakin.v0.1.1
package: anakin
properties:
- type: olm.bundle.object
  value:
    data: eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0=
- type: olm.bundle.object
  value:
    ref: objects/anakin.v0.1.1.csv.yaml
- type: olm.package
  value:
    packageName: anakin
    version: 0.1.1
relatedImages:
- image: anakin-bundle:v0.1.1
  name: bundle
schema: olm.bundle
---
myField: foobar
package: anakin
schema: custom.3
---
defaultChannel: mando
description: boba-fett operator
icon:
  base64data: PHN2ZyB2aWV3Qm94PSIwIDAgMTAwIDEwMCI+PGNpcmNsZSBjeD0iNTAiIGN5PSI1MCIgcj0iNTAiLz48L3N2Zz4=
  mediatype: image/svg+xml
name: boba-fett
schema: olm.package
---
entries:
- name: boba-fett.v1.0.0
- name: boba-fett.v2.0.0
  replaces: boba-fett.v1.0.0
name: mando
package: boba-fett
schema: olm.channel
---
image: boba-fett-bundle:v1.0.0
name: boba-fett.v1.0.0
package: boba-fett
properties:
- type: olm.bundle.object
  value:
    data: eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0=
- type: olm.bundle.object
  value:
    ref: objects/boba-fett.v1.0.0.csv.yaml
- type: olm.package
  value:
    packageName: boba-fett
    version: 1.0.0
relatedImages:
- image: boba-fett-bundle:v1.0.0
  name: bundle
schema: olm.bundle
---
image: boba-fett-bundle:v2.0.0
name: boba-fett.v2.0.0
package: boba-fett
properties:
- type: olm.bundle.object
  value:
    data: eyJraW5kIjogIkN1c3RvbVJlc291cmNlRGVmaW5pdGlvbiIsICJhcGlWZXJzaW9uIjogImFwaWV4dGVuc2lvbnMuazhzLmlvL3YxIn0=
- type: olm.bundle.object
  value:
    ref: objects/boba-fett.v2.0.0.csv.yaml
- type: olm.package
  value:
    packageName: boba-fett
    version: 2.0.0
relatedImages:
- image: boba-fett-bundle:v2.0.0
  name: bundle
schema: olm.bundle
---
myField: foobar
package: boba-fett
schema: custom.3
---
schema: custom.1
---
schema: custom.2
`,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteYAML(s.cfg, &buf)
			require.NoError(t, err)
			require.Equal(t, s.expected, buf.String())
		})
	}
}

func removeJSONWhitespace(cfg *DeclarativeConfig) {
	for ib := range cfg.Bundles {
		for ip := range cfg.Bundles[ib].Properties {
			var buf bytes.Buffer
			json.Compact(&buf, cfg.Bundles[ib].Properties[ip].Value)
			cfg.Bundles[ib].Properties[ip].Value = buf.Bytes()
		}
	}
	for io := range cfg.Others {
		var buf bytes.Buffer
		json.Compact(&buf, cfg.Others[io].Blob)
		cfg.Others[io].Blob = buf.Bytes()
	}
}

func TestWriteMermaidChannels(t *testing.T) {
	type spec struct {
		name     string
		cfg      DeclarativeConfig
		expected string
	}
	specs := []spec{
		{
			name: "Success",
			cfg:  buildValidDeclarativeConfig(true),
			expected: `<!-- PLEASE NOTE:  skipRange edges are not currently displayed -->
graph LR
  %% package "anakin"
  subgraph "anakin"
    %% channel "dark"
    subgraph anakin-dark["dark"]
      anakin-dark-anakin.v0.0.1["anakin.v0.0.1"]
      anakin-dark-anakin.v0.1.0["anakin.v0.1.0"]
      anakin-dark-anakin.v0.1.0["anakin.v0.1.0"]-- replaces --> anakin-dark-anakin.v0.0.1["anakin.v0.0.1"]
      anakin-dark-anakin.v0.1.1["anakin.v0.1.1"]
      anakin-dark-anakin.v0.1.1["anakin.v0.1.1"]-- replaces --> anakin-dark-anakin.v0.0.1["anakin.v0.0.1"]
      anakin-dark-anakin.v0.1.1["anakin.v0.1.1"]-- skips --> anakin-dark-anakin.v0.1.0["anakin.v0.1.0"]
    end
    %% channel "light"
    subgraph anakin-light["light"]
      anakin-light-anakin.v0.0.1["anakin.v0.0.1"]
      anakin-light-anakin.v0.1.0["anakin.v0.1.0"]
      anakin-light-anakin.v0.1.0["anakin.v0.1.0"]-- replaces --> anakin-light-anakin.v0.0.1["anakin.v0.0.1"]
    end
  end
  %% package "boba-fett"
  subgraph "boba-fett"
    %% channel "mando"
    subgraph boba-fett-mando["mando"]
      boba-fett-mando-boba-fett.v1.0.0["boba-fett.v1.0.0"]
      boba-fett-mando-boba-fett.v2.0.0["boba-fett.v2.0.0"]
      boba-fett-mando-boba-fett.v2.0.0["boba-fett.v2.0.0"]-- replaces --> boba-fett-mando-boba-fett.v1.0.0["boba-fett.v1.0.0"]
    end
  end
<!-- PLEASE NOTE:  skipRange edges are not currently displayed -->
`,
		},
	}
	startVersion := ""
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteMermaidChannels(s.cfg, &buf, startVersion)
			require.NoError(t, err)
			require.Equal(t, s.expected, buf.String())
		})
	}
}
