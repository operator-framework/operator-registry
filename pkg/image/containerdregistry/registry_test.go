package containerdregistry

import (
	"archive/tar"
	"testing"
)

func TestFromFilter(t *testing.T) {
	for _, tt := range []struct {
		name   string
		from   string
		header *tar.Header
		match  bool
	}{
		{
			name:   "Match/Root",
			from:   "/",
			header: &tar.Header{Name: "my.db"},
			match:  true,
		},
		{
			name:   "Match/File",
			from:   "my.db",
			header: &tar.Header{Name: "my.db"},
			match:  true,
		},
		{
			name:   "Match/Directory",
			from:   "mine",
			header: &tar.Header{Name: "mine/my.db"},
			match:  true,
		},
		{
			name:   "NoMatch/File",
			from:   "my.db",
			header: &tar.Header{Name: "your.db"},
			match:  false,
		},
		{
			name:   "NoMatch/Directory",
			from:   "mine",
			header: &tar.Header{Name: "yours/your.db"},
			match:  false,
		},
		{
			name:   "NoMatch/PartialPath",
			from:   "database/my.db",
			header: &tar.Header{Name: "yours/database/my.db"},
			match:  false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fromFilter(tt.from)(tt.header)
			if result == tt.match && err == nil {
				return
			}
			t.Fatalf("error for %s, expected match to be %t, got %t", tt.from, tt.match, result)
		})
	}

}
