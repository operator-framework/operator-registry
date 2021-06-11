package containerdregistry

import (
	"archive/tar"
	"testing"
)

func Test_fromFilter(t *testing.T) {
	type destination struct {
		dest   string
		match  bool
		header *tar.Header
	}
	var destinations = []destination{
		{
			dest:   "/",
			header: &tar.Header{Name: "root"},
			match:  true,
		},
		{
			dest:   "my-db.db",
			header: &tar.Header{Name: "my-db.db"},
			match:  true,
		},
		{
			dest:   "my-db.db",
			header: &tar.Header{Name: "rocks-db.db"},
			match:  false,
		},
		// TODO directory test cases
	}

	for _, d := range destinations {
		filter := fromFilter(d.dest)
		result, err := filter(d.header)
		if result == d.match && err == nil {
			continue
		}
		t.Fatalf("error for %s, expected match to be %t, got %t", d.dest, d.match, result)
	}
}
