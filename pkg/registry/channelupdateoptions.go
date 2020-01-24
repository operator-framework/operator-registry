package registry

import (
	"strings"
)

type Mode int

const (
	ReplacesMode = iota
	SemVerMode
	SkipPatchMode
)

func GetModeFromString(mode string) Mode {
	switch strings.ToLower(mode) {
	case "replaces":
		return ReplacesMode
	case "semver":
		return SemVerMode
	case "semver-skippatch":
		return SkipPatchMode
	default:
		return ReplacesMode
	}
}

type ChannelUpdateOptions struct {
	CSVToInsert string
}

type ChannelUpdateOption func(*ChannelUpdateOptions)

func WithCSVToInsert(csv string) ChannelUpdateOption {
	return func(o *ChannelUpdateOptions) {
		o.CSVToInsert = csv
	}
}
