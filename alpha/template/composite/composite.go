package composite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/operator-framework/operator-registry/pkg/image"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type BuilderMap map[string]Builder

type CatalogBuilderMap map[string]BuilderMap

type Template struct {
	CatalogFile      string
	ContributionFile string
	Validate         bool
	OutputType       string
	Registry         image.Registry
}

// TODO(everettraven): do we need the context here? If so, how should it be used?
func (t *Template) Render(ctx context.Context, validate bool) error {

	catalogFile, err := t.parseCatalogsSpec()
	if err != nil {
		return err
	}

	contributionFile, err := t.parseContributionSpec()
	if err != nil {
		return err
	}

	catalogBuilderMap, err := t.newCatalogBuilderMap(catalogFile.Catalogs, t.OutputType)
	if err != nil {
		return err
	}

	// TODO(everettraven): should we return aggregated errors?
	for _, component := range contributionFile.Components {
		if builderMap, ok := (*catalogBuilderMap)[component.Name]; ok {
			if builder, ok := builderMap[component.Strategy.Template.Schema]; ok {
				// run the builder corresponding to the schema
				err := builder.Build(ctx, t.Registry, component.Destination.Path, component.Strategy.Template)
				if err != nil {
					return fmt.Errorf("building component %q: %w", component.Name, err)
				}

				if validate {
					// run the validation for the builder
					err = builder.Validate(component.Destination.Path)
					if err != nil {
						return fmt.Errorf("validating component %q: %w", component.Name, err)
					}
				}
			} else {
				return fmt.Errorf("building component %q: no builder found for template schema %q", component.Name, component.Strategy.Template.Schema)
			}
		} else {
			allowedComponents := []string{}
			for k := range builderMap {
				allowedComponents = append(allowedComponents, k)
			}
			return fmt.Errorf("building component %q: component does not exist in the catalog configuration. Available components are: %s", component.Name, allowedComponents)
		}
	}
	return nil
}

func builderForSchema(schema string, builderCfg BuilderConfig) (Builder, error) {
	var builder Builder
	switch schema {
	case BasicBuilderSchema:
		builder = NewBasicBuilder(builderCfg)
	case SemverBuilderSchema:
		builder = NewSemverBuilder(builderCfg)
	case RawBuilderSchema:
		builder = NewRawBuilder(builderCfg)
	case CustomBuilderSchema:
		builder = NewCustomBuilder(builderCfg)
	default:
		return nil, fmt.Errorf("unknown schema %q", schema)
	}

	return builder, nil
}

func (t *Template) parseCatalogsSpec() (*CatalogConfig, error) {
	var tempCatalog io.ReadCloser
	catalogURI, err := url.ParseRequestURI(t.CatalogFile)
	if err != nil {
		tempCatalog, err = os.Open(t.CatalogFile)
		if err != nil {
			return nil, fmt.Errorf("opening catalog config file %q: %v", t.CatalogFile, err)
		}
		defer tempCatalog.Close()
	} else {
		tempResp, err := http.Get(catalogURI.String())
		if err != nil {
			return nil, fmt.Errorf("fetching remote catalog config file %q: %v", t.CatalogFile, err)
		}
		tempCatalog = tempResp.Body
		defer tempCatalog.Close()
	}
	catalogData := tempCatalog

	// get catalog configurations
	catalogConfig := &CatalogConfig{}
	catalogDoc := json.RawMessage{}
	catalogDecoder := yaml.NewYAMLOrJSONDecoder(catalogData, 4096)
	err = catalogDecoder.Decode(&catalogDoc)
	if err != nil {
		return nil, fmt.Errorf("decoding catalog config: %v", err)
	}
	err = json.Unmarshal(catalogDoc, catalogConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling catalog config: %v", err)
	}

	if catalogConfig.Schema != CatalogSchema {
		return nil, fmt.Errorf("catalog configuration file has unknown schema, should be %q", CatalogSchema)
	}

	return catalogConfig, nil
}

func (t *Template) parseContributionSpec() (*CompositeConfig, error) {

	compositeData, err := os.Open(t.ContributionFile)
	if err != nil {
		return nil, fmt.Errorf("opening composite config file %q: %v", t.ContributionFile, err)
	}
	defer compositeData.Close()

	// parse data to composite config
	compositeConfig := &CompositeConfig{}
	compositeDoc := json.RawMessage{}
	compositeDecoder := yaml.NewYAMLOrJSONDecoder(compositeData, 4096)
	err = compositeDecoder.Decode(&compositeDoc)
	if err != nil {
		return nil, fmt.Errorf("decoding composite config: %v", err)
	}
	err = json.Unmarshal(compositeDoc, compositeConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling composite config: %v", err)
	}

	if compositeConfig.Schema != CompositeSchema {
		return nil, fmt.Errorf("%q has unknown schema, should be %q", t.ContributionFile, CompositeSchema)
	}

	return compositeConfig, nil
}

func (t *Template) newCatalogBuilderMap(catalogs []Catalog, outputType string) (*CatalogBuilderMap, error) {

	catalogBuilderMap := make(CatalogBuilderMap)

	// setup the builders for each catalog
	setupFailed := false
	setupErrors := map[string][]string{}
	for _, catalog := range catalogs {
		errs := []string{}
		if catalog.Destination.BaseImage == "" {
			errs = append(errs, "destination.baseImage must not be an empty string")
		}

		if catalog.Destination.WorkingDir == "" {
			errs = append(errs, "destination.workingDir must not be an empty string")
		}

		// check for validation errors and skip builder creation if there are any errors
		if len(errs) > 0 {
			setupFailed = true
			setupErrors[catalog.Name] = errs
			continue
		}

		if _, ok := catalogBuilderMap[catalog.Name]; !ok {
			builderMap := make(BuilderMap)
			for _, schema := range catalog.Builders {
				builder, err := builderForSchema(schema, BuilderConfig{
					OutputType: outputType,
				})
				if err != nil {
					return nil, fmt.Errorf("getting builder %q for catalog %q: %v", schema, catalog.Name, err)
				}
				builderMap[schema] = builder
			}
			catalogBuilderMap[catalog.Name] = builderMap
		}
	}

	// if there were errors validating the catalog configuration then exit
	if setupFailed {
		//build the error message
		var errMsg string
		for cat, errs := range setupErrors {
			errMsg += fmt.Sprintf("\nCatalog %v:\n", cat)
			for _, err := range errs {
				errMsg += fmt.Sprintf("  - %v\n", err)
			}
		}
		return nil, fmt.Errorf("catalog configuration file field validation failed: %s", errMsg)
	}

	return &catalogBuilderMap, nil
}
