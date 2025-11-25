package template

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

var tr = NewTemplateRegistry()

// GetTemplateRegistry returns the global template registry
func GetTemplateRegistry() *TemplateRegistry {
	return tr
}

func (r *TemplateRegistry) HelpText() string {
	var help strings.Builder
	supportedTypes := r.GetSupportedTypes()
	help.WriteString("\n")
	tabber := tabwriter.NewWriter(&help, 0, 0, 1, ' ', 0)
	for _, item := range supportedTypes {
		fmt.Fprintf(tabber, " - %s\n", item)
	}
	tabber.Flush()
	return help.String()
}
