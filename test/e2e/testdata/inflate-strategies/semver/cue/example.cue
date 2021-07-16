pkg: {
		name: "test"
		defaultChannel: "stable"
		...
}
bundles: [
	{
		version: "1.1.0"
		image: "test.v1.0.0"
		channels: ["stable", "fast"]
	},
	{
		version: "1.0.0-1+2"
		image: "test.v1.0.0"
		channels: ["stable", "fast"]
	},
	{
		version: "1.0.1"
		image: "test.v1.0.1"
		channels: ["stable", "fast"]
	},
	{
		version: "1.0.0-2+2"
		image: "test.v1.0.0"
		channels: ["stable", "fast"]
	},
]
