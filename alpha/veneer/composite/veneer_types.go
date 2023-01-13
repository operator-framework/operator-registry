package composite

import "encoding/json"

type VeneerDef struct {
	Schema string
	Config json.RawMessage
}

type BasicVeneerConfig struct {
	Input  string
	Output string
}

type SemverVeneerConfig struct {
	Input  string
	Output string
}

type RawVeneerConfig struct {
	Input  string
	Output string
}

type CustomVeneerConfig struct {
	Command []string
}
