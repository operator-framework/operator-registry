pkg: {
	name:           "prometheus"
	defaultChannel: "alpha"
	...
}

bundles: [
{
	version: "0.14.0"
	image:   "docker.io/anik120/e2e-bundle:9prd26"
	provides: {
		group:   "monitoring.coreos.com"
		version: "v1"
		kinds: ["Alertmanager", "ServiceMonitor", "PrometheusRule", "Prometheus"]
	}
},
{
	version: "0.15.0"
	image:   "docker.io/anik120/e2e-bundle:9prd26"
	provides: {
		group:   "monitoring.coreos.com"
		version: "v1"
		kinds: ["Alertmanager", "ServiceMonitor", "PrometheusRule", "Prometheus"]
	}
}
]

edges: {
	alpha: {
		"0.14.0": "0.15.0"
	}
	stable: {
		"0.14.0": ""
	}
}
