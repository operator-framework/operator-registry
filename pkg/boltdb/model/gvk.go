package model

const GvkCapability = OperatorsNamespace + "gvk"

func init() {
	capabilityTypeForName[GvkCapability] = func() interface{} { return &Api{} }
	requirementTypeForName[GvkCapability] = func() interface{} { return &ApiEqualitySelector{} }
}

type Api struct {
	Group   string
	Version string
	Kind    string
	Plural  string
}

func NewApiCapability(api *Api) Capability {
	return Capability{
		Name:  GvkCapability,
		Value: api,
	}
}

type ApiEqualitySelector struct {
	Group   string
	Version string
	Kind    string
	Plural  string
}

func NewApiEqualityRequirement(api *ApiEqualitySelector) Requirement {
	return Requirement{
		Name:     GvkCapability,
		Selector: api,
		Optional: false,
	}
}
