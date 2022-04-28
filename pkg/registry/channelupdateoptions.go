package registry

import (
	"fmt"
	"strings"
)

type Mode string

const (
	ReplacesMode  Mode = "replaces"
	SemVerMode    Mode = "semver"
	SkipPatchMode Mode = "semver-skippatch"
	FBCMode       Mode = "fbc"
)

func GetModeFromString(mode string) (Mode, error) {
	switch strings.ToLower(mode) {
	case "replaces":
		return ReplacesMode, nil
	case "semver":
		return SemVerMode, nil
	case "semver-skippatch":
		return SkipPatchMode, nil
	case "fbc":
		return FBCMode, nil
	default:
		return "", fmt.Errorf("Invalid channel update mode %s specified", mode)
	}
}
