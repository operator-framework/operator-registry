package composite

import "encoding/json"

type TemplateDefinition struct {
	Schema string
	Config json.RawMessage
}

type BasicConfig struct {
	Input  string
	Output string
}

type SemverConfig struct {
	Input  string
	Output string
}

type RawConfig struct {
	Input  string
	Output string
}

type CustomConfig struct {
	Command string
	Args    []string
	Output  string
}
