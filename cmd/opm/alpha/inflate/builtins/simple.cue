data: {
	pkg: {
		name: string
		defaultChannel: string
		...
	}
	...
}

out: [
  {"schema": "olm.package", ...} & data.pkg
] +
[
  for b in data.bundles {
    "schema": "olm.bundle"
    "name": "\(data.pkg.name).\(b.version)"
    "package": "\(data.pkg.name)"
    "image": b.image
    "properties": [{
        "type": "olm.package",
        "value": {
            "packageName": data.pkg.name
            "version": b.version
        }
    }] +
    [
        for k in b.provides.kinds {
            "type": "olm.gvk"
            "value": {
              "group": b.provides.group
              "version": b.provides.version
              "kind": k
            }
        }
    ] +
    [
        for channel, edge in data.edges, for from,to in edge if b.version == from || b.version == to {
            "type": "olm.channel"
            "value": {
                "name": channel
                if b.version == to {
                    "replaces": from
                }
            },
        }
    ]
  }
]