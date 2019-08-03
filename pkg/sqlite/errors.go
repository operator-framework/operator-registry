package sqlite

import (
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type missingReplaceeError struct {
	pkg         string
	channel     string
	replacement string
	replacee    string
}

func (m *missingReplaceeError) Error() string {
	return fmt.Sprintf("package/channel %s/%s csv %s specifies replaces that could not be found: %s", m.pkg, m.channel, m.replacement, m.replacee)
}

func newMissingReplaceeError(pkg, channel, replacement, replacee string) *missingReplaceeError {
	return &missingReplaceeError{
		pkg:         pkg,
		channel:     channel,
		replacement: replacement,
		replacee:    replacee,
	}
}

type missingChannelEntryError struct {
	pkg     string
	channel string
	entry   string
}

func (m *missingChannelEntryError) Error() string {
	return fmt.Sprintf("package/channel %s/%s specifies entry csv that could not be found: %s", m.pkg, m.channel, m.entry)
}

func newMissingChannelEntryError(pkg, channel, entry string) *missingChannelEntryError {
	return &missingChannelEntryError{
		pkg:     pkg,
		channel: channel,
		entry:   entry,
	}
}

type noDefaultChannelDefinedError struct {
	pkg string
}

func (n *noDefaultChannelDefinedError) Error() string {
	return fmt.Sprintf("package %s does not define a default channel", n.pkg)
}

func newNoDefaultChannelDefinedError(pkg string) *noDefaultChannelDefinedError {
	return &noDefaultChannelDefinedError{pkg}
}

const sqlLoadErrorType registry.LoadErrorType = "sql"

func newSQLLoadError(err error) *registry.LoadError {
	return registry.NewLoadError(sqlLoadErrorType, err)
}

const directoryLoadErrorType registry.LoadErrorType = "directory"

func newDirectoryLoadError(err error) *registry.LoadError {
	return registry.NewLoadError(directoryLoadErrorType, err)
}

const configMapLoadErrorType registry.LoadErrorType = "configmap"

type configMapError struct {
	err error
}

func newConfigMapLoadError(err error) *registry.LoadError {
	return registry.NewLoadError(configMapLoadErrorType, err)
}
