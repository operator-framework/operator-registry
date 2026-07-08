package action

import (
	"fmt"
	"io"

	"github.com/h2non/filetype"
	"github.com/h2non/go-is-svg"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

type Init struct {
	Package           string
	DefaultChannel    string
	DescriptionReader io.Reader
	IconReader        io.Reader
}

func (i Init) Run() (*declcfg.Package, error) {
	pkg := &declcfg.Package{
		// TODO(joelanford): Use a constant for "olm.package"
		Schema:         "olm.package",
		Name:           i.Package,
		DefaultChannel: i.DefaultChannel,
	}
	if i.DescriptionReader != nil {
		descriptionData, err := io.ReadAll(i.DescriptionReader)
		if err != nil {
			return nil, fmt.Errorf("read description: %v", err)
		}
		pkg.Description = string(descriptionData)
	}

	if i.IconReader != nil {
		icon, err := processIcon(i.IconReader)
		if err != nil {
			return nil, err
		}
		pkg.Icon = icon
	}
	return pkg, nil
}

func processIcon(iconReader io.Reader) (*declcfg.Icon, error) {
	iconData, err := io.ReadAll(iconReader)
	if err != nil {
		return nil, fmt.Errorf("read icon: %v", err)
	}

	// Try filetype detection first
	iconType, err := filetype.Match(iconData)
	if err != nil {
		return nil, fmt.Errorf("detect icon mediatype: %v", err)
	}

	var mediaType string
	// If filetype didn't detect it, check if it's SVG
	if iconType.MIME.Value == "" {
		if issvg.Is(iconData) {
			mediaType = "image/svg+xml"
		} else {
			return nil, fmt.Errorf("detected invalid type %q: not an image", iconType.MIME.Value)
		}
	} else {
		if iconType.MIME.Type != "image" {
			return nil, fmt.Errorf("detected invalid type %q: not an image", iconType.MIME.Value)
		}
		mediaType = iconType.MIME.Value
	}

	return &declcfg.Icon{
		Data:      iconData,
		MediaType: mediaType,
	}, nil
}
