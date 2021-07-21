package builder

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"strings"
)

func (i *imageBuilder) SetLabels(ctx context.Context, platformMatcher platforms.MatchComparer, labels map[string]string) error {
	ctx = ensureNamespace(ctx)

	var action string
	if len(labels) > 0 {
		action := "LABEL"
		for k, v := range labels {
			action = fmt.Sprintf("%s %s=%s", action, k, v)
		}
	}
	return i.updateConfig(ctx, platformMatcher, func(config *ocispec.Image) (bool, string, error) {
		var changed bool
		if len(labels) > 0 && len(config.Config.Labels) == 0 {
			config.Config.Labels = map[string]string{}
		}
		for name, value := range labels {
			if value != config.Config.Labels[name] {
				config.Config.Labels[name] = value
				changed = true
			}
		}
		return changed, action, nil
	})
}

func (i *imageBuilder) ExposePorts(ctx context.Context, platformMatcher platforms.MatchComparer, port string) error {
	ctx = ensureNamespace(ctx)
	action := fmt.Sprintf("EXPOSE %s", port)
	return i.updateConfig(ctx, platformMatcher, func(config *ocispec.Image) (bool, string, error) {
		var changed bool
		if len(config.Config.ExposedPorts) == 0 {
			config.Config.ExposedPorts = map[string]struct{}{}
		}
		if _, ok := config.Config.ExposedPorts[port]; !ok {
			config.Config.ExposedPorts[port] = struct{}{}
			changed = true
		}
		return changed, action, nil
	})
}

func (i *imageBuilder) SetEntrypoint(ctx context.Context, platformMatcher platforms.MatchComparer, entrypoint []string) error {
	ctx = ensureNamespace(ctx)
	var action string
	if len(entrypoint) > 0 {
		action = fmt.Sprintf("ENTRYPOINT [\"%s\"]", strings.Join(entrypoint, "\",\""))
	}
	return i.updateConfig(ctx, platformMatcher, func(config *ocispec.Image) (bool, string, error) {
		var changed bool
		if len(config.Config.Entrypoint) != len(entrypoint) {
			changed = true
		} else {
			for i := range entrypoint {
				if config.Config.Entrypoint[i] != entrypoint[i] {
					changed = true
					break
				}
			}
		}
		config.Config.Entrypoint = entrypoint
		return changed, action, nil
	})
}

func (i *imageBuilder) SetCmd(ctx context.Context, platformMatcher platforms.MatchComparer, cmd []string) error {
	ctx = ensureNamespace(ctx)
	var action string
	if len(cmd) > 0 {
		action = fmt.Sprintf("CMD [\"%s\"]", strings.Join(cmd, "\",\""))
	}
	return i.updateConfig(ctx, platformMatcher, func(config *ocispec.Image) (bool, string, error) {
		var changed bool
		if len(config.Config.Cmd) != len(cmd) {
			changed = true
		} else {
			for i := range cmd {
				if config.Config.Cmd[i] != cmd[i] {
					changed = true
					break
				}
			}
		}
		config.Config.Cmd = cmd
		return changed, action, nil
	})
}

func (i *imageBuilder) SetUser(ctx context.Context, platformMatcher platforms.MatchComparer, user string) error {
	ctx = ensureNamespace(ctx)
	var action string
	action = fmt.Sprintf("USER %s", user)

	return i.updateConfig(ctx, platformMatcher, func(config *ocispec.Image) (bool, string, error) {
		var changed bool
		if config.Config.User != user {
			changed = true
			config.Config.User = user
		}
		return changed, action, nil
	})
}
