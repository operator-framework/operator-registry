package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

type serve struct {
	configDir string
	addr      string
	logger    *logrus.Entry
}

func NewCmd() *cobra.Command {
	s := serve{
		logger: logrus.NewEntry(logrus.New()),
	}
	cmd := &cobra.Command{
		Use:   "http <source_path>",
		Short: "serve declarative configs",
		Long: `This command serves declarative configs via a GRPC server.

NOTE: The declarative config directory is loaded by the serve command at
startup. Changes made to the declarative config after the this command starts
will not be reflected in the served content.
`,
		Args: cobra.ExactArgs(1),
		PreRun: func(_ *cobra.Command, args []string) {
			s.configDir = args[0]
		},
		Run: func(cmd *cobra.Command, _ []string) {
			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			if err := s.run(ctx); err != nil {
				s.logger.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVar(&s.addr, "addr", "127.0.0.1:8080", "address to serve on")
	return cmd
}

func (s *serve) run(ctx context.Context) error {

	// Create a temporary file that we'll serve from.
	catalogFile, err := os.CreateTemp("", "catalog-*.json")
	if err != nil {
		return err
	}
	defer os.RemoveAll(catalogFile.Name())

	// Write the catalog to the temporary file as a JSON stream.
	catalogFS := os.DirFS(s.configDir)
	if err := declcfg.ToJSONStream(catalogFS, catalogFile); err != nil {
		return err
	}

	// Generate in-memory indices for the catalog for all combinations of
	// schema, package, and name.
	catalogFile.Seek(0, io.SeekStart)
	idxs, err := newIndices(catalogFile)
	if err != nil {
		return err
	}

	// Compute a hash of the catalog file contents.
	catalogFile.Seek(0, io.SeekStart)
	catalogHasher := fnv.New64a()
	if _, err := io.Copy(catalogHasher, catalogFile); err != nil {
		return err
	}
	catalogSum := fmt.Sprintf("%x", catalogHasher.Sum(nil))
	if err := catalogFile.Close(); err != nil {
		return err
	}

	// Create the http server
	srv := http.Server{
		Addr: s.addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only GET and HEAD are supported.
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// All responses are JSON and have a hash header for the entire catalog.
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Catalogd-Catalog-Hash", catalogSum)

			// Get the relevant query params and add them to a fields map
			qParams := r.URL.Query()
			schema := qParams.Get("schema")
			pkg := qParams.Get("package")
			name := qParams.Get("name")

			fields := logrus.Fields{}
			if schema != "" {
				fields["schema"] = schema
			}
			if pkg != "" {
				fields["package"] = pkg
			}
			if name != "" {
				fields["name"] = name
			}
			s.logger.WithFields(fields).Info("handling request")

			// Open the catalog file and defer closing it.
			f, err := os.Open(catalogFile.Name())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()

			var (
				fbcReader io.Reader
				etag      string
			)

			if len(fields) == 0 {
				// If there were no query params, serve the entire catalog file.
				fbcReader, etag, err = f, catalogSum, nil
			} else {
				// Otherwise, serve the relevant subset of the catalog file, using
				// a reader tailored to the query params.
				fbcReader, etag, err = idxs.readerForParams(f, qParams)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Set the ETag header
			w.Header().Set("ETag", etag)

			// If the client already has the latest version.
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			// If the client is requesting a HEAD, we're done.
			if r.Method == http.MethodHead {
				return
			}

			io.Copy(w, fbcReader)
		}),
	}

	s.logger.WithFields(logrus.Fields{"configs": s.configDir, "addr": s.addr}).Info("listening")
	serveErr := make(chan error)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		s.logger.Info("shutting down")
		return srv.Shutdown(ctx)
	case err := <-serveErr:
		return err
	}
}

type indexData struct {
	locations []location
	hasher    hash.Hash64
}

type location struct {
	begin int64
	end   int64
}

func updateIndices(r io.Reader, indices ...index) error {
	dec := json.NewDecoder(r)
	startOffset := int64(0)
	for {
		var m declcfg.Meta
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		// TODO: This is a hack to make sure olm.package blobs are returned
		//   in the response when the package name is requested.
		if m.Schema == "olm.package" {
			m.Package = m.Name
		}

		for _, idx := range indices {
			idx.Update(m, startOffset, dec.InputOffset())
		}
		startOffset = dec.InputOffset()
	}
	return nil
}

type index interface {
	Update(m declcfg.Meta, begin int64, end int64)
	Get(keys ...string) (*indexData, error)
}

type schemaIndex map[string]indexData

func (idx schemaIndex) Update(m declcfg.Meta, begin int64, end int64) {
	data := idx[m.Schema]
	data.locations = append(data.locations, location{begin, end})
	if data.hasher == nil {
		data.hasher = fnv.New64a()
	}
	data.hasher.Write(m.Blob)
	idx[m.Schema] = data
}
func (idx schemaIndex) Get(keys ...string) (*indexData, error) {
	if len(keys) != 1 {
		return nil, fmt.Errorf("schema index only supports one key")
	}
	v := idx[keys[0]]
	return &v, nil
}

type packageIndex map[string]indexData

func (idx packageIndex) Update(m declcfg.Meta, begin int64, end int64) {
	data := idx[m.Package]
	data.locations = append(data.locations, location{begin, end})
	if data.hasher == nil {
		data.hasher = fnv.New64a()
	}
	data.hasher.Write(m.Blob)
	idx[m.Package] = data
}

func (idx packageIndex) Get(keys ...string) (*indexData, error) {
	if len(keys) != 1 {
		return nil, fmt.Errorf("package index only supports one key")
	}
	v := idx[keys[0]]
	return &v, nil
}

type nameIndex map[string]indexData

func (idx nameIndex) Update(m declcfg.Meta, begin int64, end int64) {
	data := idx[m.Name]
	data.locations = append(data.locations, location{begin, end})
	if data.hasher == nil {
		data.hasher = fnv.New64a()
	}
	data.hasher.Write(m.Blob)
	idx[m.Name] = data
}

func (idx nameIndex) Get(keys ...string) (*indexData, error) {
	if len(keys) != 1 {
		return nil, fmt.Errorf("name index only supports one key")
	}
	v := idx[keys[0]]
	return &v, nil
}

type schemaAndPackageIndex map[string]map[string]indexData

func (idx schemaAndPackageIndex) Update(m declcfg.Meta, begin int64, end int64) {
	if _, ok := idx[m.Schema]; !ok {
		idx[m.Schema] = map[string]indexData{}
	}
	data := idx[m.Schema][m.Package]
	data.locations = append(data.locations, location{begin, end})
	if data.hasher == nil {
		data.hasher = fnv.New64a()
	}
	data.hasher.Write(m.Blob)
	idx[m.Schema][m.Package] = data
}

func (idx schemaAndPackageIndex) Get(keys ...string) (*indexData, error) {
	if len(keys) != 2 {
		return nil, fmt.Errorf("schema and package index only supports two keys")
	}
	v := idx[keys[0]][keys[1]]
	return &v, nil
}

type schemaAndNameIndex map[string]map[string]indexData

func (idx schemaAndNameIndex) Update(m declcfg.Meta, begin int64, end int64) {
	if _, ok := idx[m.Schema]; !ok {
		idx[m.Schema] = map[string]indexData{}
	}
	data := idx[m.Schema][m.Name]
	data.locations = append(data.locations, location{begin, end})
	if data.hasher == nil {
		data.hasher = fnv.New64a()
	}
	data.hasher.Write(m.Blob)
	idx[m.Schema][m.Name] = data
}

func (idx schemaAndNameIndex) Get(keys ...string) (*indexData, error) {
	if len(keys) != 2 {
		return nil, fmt.Errorf("schema and name index only supports two keys")
	}
	v := idx[keys[0]][keys[1]]
	return &v, nil
}

type packageAndNameIndex map[string]map[string]indexData

func (idx packageAndNameIndex) Update(m declcfg.Meta, begin int64, end int64) {
	if _, ok := idx[m.Package]; !ok {
		idx[m.Package] = map[string]indexData{}
	}
	data := idx[m.Package][m.Name]
	data.locations = append(data.locations, location{begin, end})
	if data.hasher == nil {
		data.hasher = fnv.New64a()
	}
	data.hasher.Write(m.Blob)
	idx[m.Package][m.Name] = data
}

func (idx packageAndNameIndex) Get(keys ...string) (*indexData, error) {
	if len(keys) != 2 {
		return nil, fmt.Errorf("package and name index only supports two keys")
	}
	v := idx[keys[0]][keys[1]]
	return &v, nil
}

type schemaPackageAndNameIndex map[string]map[string]map[string]indexData

func (idx schemaPackageAndNameIndex) Update(m declcfg.Meta, begin int64, end int64) {
	if _, ok := idx[m.Schema]; !ok {
		idx[m.Schema] = map[string]map[string]indexData{}
	}
	if _, ok := idx[m.Schema][m.Package]; !ok {
		idx[m.Schema][m.Package] = map[string]indexData{}
	}
	data := idx[m.Schema][m.Package][m.Name]
	data.locations = append(data.locations, location{begin, end})
	if data.hasher == nil {
		data.hasher = fnv.New64a()
	}
	data.hasher.Write(m.Blob)
	idx[m.Schema][m.Package][m.Name] = data
}

func (idx schemaPackageAndNameIndex) Get(keys ...string) (*indexData, error) {
	if len(keys) != 3 {
		return nil, fmt.Errorf("schema, package, and name index only supports three keys")
	}
	v := idx[keys[0]][keys[1]][keys[2]]
	return &v, nil
}

func indexReader(r io.ReaderAt, idx index, keys ...string) (io.Reader, string, error) {
	data, err := idx.Get(keys...)
	if err != nil {
		return nil, "", err
	}
	readers := make([]io.Reader, 0, len(data.locations))
	for _, loc := range data.locations {
		readers = append(readers, io.NewSectionReader(r, loc.begin, loc.end-loc.begin))
	}
	return io.MultiReader(readers...), fmt.Sprintf("%x", data.hasher.Sum(nil)), nil
}

type indices struct {
	schemaIndex               schemaIndex
	packageIndex              packageIndex
	nameIndex                 nameIndex
	schemaAndPackageIndex     schemaAndPackageIndex
	schemaAndNameIndex        schemaAndNameIndex
	packageAndNameIndex       packageAndNameIndex
	schemaPackageAndNameIndex schemaPackageAndNameIndex
}

type readReaderAt interface {
	io.ReaderAt
	io.Reader
}

func newIndices(r readReaderAt) (*indices, error) {
	i := &indices{
		schemaIndex:               schemaIndex{},
		packageIndex:              packageIndex{},
		nameIndex:                 nameIndex{},
		schemaAndPackageIndex:     schemaAndPackageIndex{},
		schemaAndNameIndex:        schemaAndNameIndex{},
		packageAndNameIndex:       packageAndNameIndex{},
		schemaPackageAndNameIndex: schemaPackageAndNameIndex{},
	}
	if err := updateIndices(r,
		&i.schemaIndex,
		&i.packageIndex,
		&i.nameIndex,
		&i.schemaAndPackageIndex,
		&i.schemaAndNameIndex,
		&i.packageAndNameIndex,
		&i.schemaPackageAndNameIndex,
	); err != nil {
		return nil, err
	}
	return i, nil
}

func (i indices) readerForParams(r readReaderAt, params url.Values) (io.Reader, string, error) {
	var (
		schema = params.Get("schema")
		pkg    = params.Get("package")
		name   = params.Get("name")
	)

	switch {
	case schema != "" && pkg != "" && name != "":
		return indexReader(r, i.schemaPackageAndNameIndex, schema, pkg, name)
	case schema != "" && pkg != "":
		return indexReader(r, i.schemaAndPackageIndex, schema, pkg)
	case schema != "" && name != "":
		return indexReader(r, i.schemaAndNameIndex, schema, name)
	case pkg != "" && name != "":
		return indexReader(r, i.packageAndNameIndex, pkg, name)
	case schema != "":
		return indexReader(r, i.schemaIndex, schema)
	case pkg != "":
		return indexReader(r, i.packageIndex, pkg)
	case name != "":
		return indexReader(r, i.nameIndex, name)
	default:
		return nil, "", fmt.Errorf("no index found for params: %v", params)
	}
}
