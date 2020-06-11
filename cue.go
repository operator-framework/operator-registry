package main

import (
	"io/ioutil"
	"log"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/encoding/protobuf"
)

func main() {
	file, err := protobuf.Extract("pkg/api/registry.proto", nil, &protobuf.Config{})
	if err != nil {
		log.Fatal(err)
	}

	b, _ := format.Node(file)
	ioutil.WriteFile("out.cue", b, 0644)

	r := cue.Runtime{}
	previnst, err := r.Compile("out_prev.cue", nil)
	if err != nil {
		log.Fatal("error compiling prev: ", err)
	}
	inst, err := r.Compile("out.cue", nil)
	if err != nil {
		log.Fatal("error compiling current: ", err)
	}
	var errs []error
	it, err := previnst.Value().Fields(cue.Definitions(true))
	if err != nil {
		log.Fatal(err)
	}
	for it.Next() {
		if !it.IsDefinition() {
			continue
		}
		def, _ := it.Value().Label()
		curr := inst.Value().LookupDef(def)
		if err:= curr.Subsume(it.Value()); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		log.Fatal("backwards incompatible changes: ", errs)
	}
	return
}