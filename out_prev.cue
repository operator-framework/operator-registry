package api

#Channel: {
	name?:    string @protobuf(1)
	csvName?: string @protobuf(2)
}

#PackageName: {
	name?: string @protobuf(1)
}

#Package: {
	name?: string @protobuf(1)
	channels?: [...#Channel] @protobuf(2)
	defaultChannelName?: string @protobuf(3)
}

#GroupVersionKind: {
	group?:   string @protobuf(1)
	version?: string @protobuf(2)
	kind?:    string @protobuf(3)
	plural?:  string @protobuf(4)
}

#Bundle: {
	csvName?:     string @protobuf(1)
	packageName?: string @protobuf(2)
	channelName?: string @protobuf(3)
	csvJson?:     string @protobuf(4)
	object?: [...string] @protobuf(5)
	bundlePath?: string @protobuf(6)
	providedApis?: [...#GroupVersionKind] @protobuf(7)
	requiredApis?: [...#GroupVersionKind] @protobuf(8)
	version?:   string @protobuf(9)
	skipRange?: string @protobuf(10)
}

#ChannelEntry: {
	packageName?: string @protobuf(1)
	channelName?: string @protobuf(2)
	bundleName?:  string @protobuf(3)
	replaces?:    string @protobuf(4)
}

#ListPackageRequest: {
}

#GetPackageRequest: {
	name?: string @protobuf(1)
}

#GetBundleRequest: {
	pkgName?:     string @protobuf(1)
	channelName?: string @protobuf(2)
	csvName?:     string @protobuf(3)
}

#GetBundleInChannelRequest: {
	pkgName?:     string @protobuf(1)
	channelName?: string @protobuf(2)
}

#GetAllReplacementsRequest: {
	csvName?: string @protobuf(1)
}

#GetReplacementRequest: {
	csvName?:     string @protobuf(1)
	pkgName?:     string @protobuf(2)
	channelName?: string @protobuf(3)
}

#GetAllProvidersRequest: {
	group?:   string @protobuf(1)
	version?: string @protobuf(2)
	kind?:    string @protobuf(3)
	plural?:  string @protobuf(4)
}

#GetLatestProvidersRequest: {
	group?:   string @protobuf(1)
	version?: string @protobuf(2)
	kind?:    string @protobuf(3)
	plural?:  string @protobuf(4)
}

#GetDefaultProviderRequest: {
	group?:   string @protobuf(1)
	version?: string @protobuf(2)
	kind?:    string @protobuf(3)
	plural?:  string @protobuf(4)
}
