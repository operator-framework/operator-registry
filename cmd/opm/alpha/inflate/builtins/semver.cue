import (
  "strings"
  "strconv"
  "list"
)


// builds a graph based on semver
//  - X.Y.Z skips X.Y.(<Z) (anyone on Y is updated to the latest Z on that Y, no one steps through an older Z)
//  - X.Y.(latest Z) replaces X.(Y-1).(latest Z)

//todo: more validation
#semver: {
  version: string
  components: {
    _splitbuild: strings.Split(version, "+")
    _splitpre: strings.Split(_splitbuild[0], "-")
    _splitv: strings.Split(_splitpre[0], ".")
    build: *strings.Split(_splitbuild[1], ".") | [""]
    prerelease: *strings.Split(_splitpre[1], ".") | [""]
    major: strconv.Atoi(_splitv[0])
    minor: strconv.Atoi(_splitv[1])
    patch: strconv.Atoi(_splitv[2])
  }
}

#bundle: {
		version: string
		image:  string
		channels: [...string]
		name: "\(data.pkg.name).v\(version)"
		_version: version
		_semver: (#semver & {version: _version}).components
}

// data is the input schema
data: {
	pkg: {
		name: string
		defaultChannel: string
		...
	}
	bundles: [...#bundle]
	...
}

#property: {
	type: string
	value: {...} | [...] | string | number
}

#blob: {
	schema: string
	properties: [...#property]
	...
}

// compare a list component-wise. used for prerelease comparison
#listLess: {
    a: [...string]
    b: [...string]
    less: list.Contains([
      for i, j in a if i<len(b) {
         a[i]<b[i]
      }
    ], true)
}

// compare two semver versions
#semverLess: {
  x: string
  y: string
  x_semver: #semver & {version: x}
  y_semver: #semver & {version: y}
  prereleaseLess: #listLess & {a: y_semver.components.prerelease, b: x_semver.components.prerelease}
  less: y_semver.components.major < x_semver.components.major ||
    ( y_semver.components.major == x_semver.components.major &&  y_semver.components.minor < x_semver.components.minor) ||
    ( y_semver.components.major == x_semver.components.major &&  y_semver.components.minor == x_semver.components.minor &&  y_semver.components.patch < x_semver.components.patch) ||
    ( y_semver.components.major == x_semver.components.major &&  y_semver.components.minor == x_semver.components.minor &&  y_semver.components.patch == x_semver.components.patch && prereleaseLess.less)
}

#sortedBundles: list.Sort(data.bundles, {
  x: #bundle,
  y: #bundle,
  prereleaseLess: #listLess & {a: y._semver.prerelease, b: x._semver.prerelease}
  less:
    y._semver.major < x._semver.major ||
    (y._semver.major == x._semver.major && y._semver.minor < x._semver.minor) ||
    (y._semver.major == x._semver.major && y._semver.minor == x._semver.minor && y._semver.patch < x._semver.patch) ||
    (y._semver.major == x._semver.major && y._semver.minor == x._semver.minor && y._semver.patch == x._semver.patch && prereleaseLess.less)
})

// skip anything below the version that has the same X.Y value
// replace the next lowest thing without the same X.Y value
#edgesForBundle: {
    _version: string,
    _channel: string
    semver: #semver & {version: _version}
    _skippedBundles: [for b in #sortedBundles if semver.components.major == b._semver.major &&
                                      semver.components.minor == b._semver.minor &&
                                      (#semverLess & {x: _version, y: b.version}).less {b}]
    skips: [for b in _skippedBundles {
            type: "olm.skips"
            value: b.name
    }]
    replaces: *([[for b in #sortedBundles if !list.Contains(_skippedBundles, b) && (#semverLess & {x: _version, y: b.version}).less && list.Contains(b.channels, _channel) {
        type: "olm.channel"
        value: {
            name: _channel
            replaces: b.name
        }
    }][0]]) | [{
        type: "olm.channel"
        value: {
            name: _channel
            replaces: ""
        }
    }]
}


out: [...#blob]

// out is the output schema; sources its data from `data`
out: [
  {schema: "olm.package", ...} & data.pkg
] +
[
  for b in data.bundles, for c in b.channels {
    schema: "olm.bundle"
    name: b.name
    "package": "\(data.pkg.name)"
    image: b.image
    _edges: #edgesForBundle & {_version: b.version, _channel: c}
    properties: list.Concat([
      [{
        type: "olm.package",
        value: {
            packageName: data.pkg.name
            version: b.version
        }
      }],
      _edges.skips,
      _edges.replaces
    ])
  }
]