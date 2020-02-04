package model

import (
	"encoding/json"
	"github.com/mitchellh/mapstructure"
)

var Codec = new(capCodec)

var capabilityTypeForName = map[string]func() interface{} {}
var requirementTypeForName = map[string]func() interface{} {}

type capCodec int

func (j capCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j capCodec) Unmarshal(b []byte, v interface{}) error {
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}

	o, ok := v.(*OperatorBundle)
	if ok {
		return j.UnmarshalBundle(o)
	}

	c, ok := v.(*Capability)
	if ok {
		return j.UnmarshalCapability(c)
	}

	r, ok := v.(*Requirement)
	if ok {
		return j.UnmarshalRequirement(r)
	}

	return nil
}

func (j capCodec) UnmarshalBundle(o *OperatorBundle) error {
	for i := range o.Capabilities{
		if err := j.UnmarshalCapability(&o.Capabilities[i]); err != nil {
			return err
		}
	}
	for i, r := range o.Requirements {
		constructor, ok := requirementTypeForName[r.Name]
		if !ok {
			continue
		}
		o.Requirements[i].Selector = constructor()
		if err := mapstructure.Decode(r.Selector, o.Requirements[i].Selector); err != nil {
			return err
		}
	}
	return nil
}

func (j capCodec) UnmarshalCapability(c *Capability) error {
	constructor, ok := capabilityTypeForName[c.Name]
	if !ok {
		return nil
	}
	val := c.Value
	c.Value = constructor()

	if err := mapstructure.Decode(val, &c.Value); err != nil {
		return err
	}

	return nil
}

func (j capCodec) UnmarshalRequirement(r *Requirement) error {
	constructor, ok := requirementTypeForName[r.Name]
	if !ok {
		return nil
	}
	sel := r.Selector
	r.Selector = constructor()

	if err := mapstructure.Decode(sel, &r.Selector); err != nil {
		return err
	}
	return nil
}

func (j capCodec) Name() string {
	return "capability"
}