package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/itchyny/gojq"
	"github.com/karrick/godirwalk"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/pkg/lib/dns"
	"github.com/operator-framework/operator-registry/pkg/lib/log"
)

type stream struct {
	configDir string

	port           string
	terminationLog string
	debug          bool

	logger *logrus.Entry
}

func NewCmd() *cobra.Command {
	logger := logrus.New()
	s := stream{
		logger: logrus.NewEntry(logger),
	}
	cmd := &cobra.Command{
		Use:   "stream <source_path>",
		Short: "serve declarative configs",
		Long:  `serve declarative configs via http`,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			s.configDir = args[0]
			if s.debug {
				logger.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return s.run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&s.debug, "debug", false, "enable debug logging")
	cmd.Flags().StringVarP(&s.port, "port", "p", "80", "port number to serve on")
	cmd.Flags().StringVarP(&s.terminationLog, "termination-log", "t", "/dev/termination-log", "path to a container termination log file")
	return cmd
}

func (s *stream) run(ctx context.Context) error {
	// Immediately set up termination log
	err := log.AddDefaultWriterHooks(s.terminationLog)
	if err != nil {
		s.logger.WithError(err).Warn("unable to set termination log path")
	}

	// Ensure there is a default nsswitch config
	if err := dns.EnsureNsswitch(); err != nil {
		s.logger.WithError(err).Warn("unable to write default nsswitch config")
	}

	s.logger = s.logger.WithFields(logrus.Fields{"configs": s.configDir, "port": s.port})

	http.HandleFunc("/blobs", func(w http.ResponseWriter, r *http.Request) {
		var filter FilterFunc
		q := r.URL.Query()["q"]
		if len(q) > 0 {
			query, err := gojq.Parse(q[0])
			if err != nil {
				http.Error(w, fmt.Sprintf("bad query %q: %v", q[0], err), http.StatusInternalServerError)
				return
			}
			filter = Query(query)
		} else {
			filter = All
		}

		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		blobs := make(chan []byte)
		errs := make(chan error)
		done := make(chan struct{})
		go streamBlobs(s.configDir, filter, blobs, errs, done)

		func() {
			for {
				select {
				case b := <-blobs:
					if _, err := w.Write(b); err != nil {
						logrus.Error(err)
					}
					f.Flush()
				case e := <-errs:
					logrus.Error(e)
				case <-done:
					return
				}
			}
		}()
	})
	logrus.Fatal(http.ListenAndServe(":8080", nil))
	return nil
}

type Meta struct {
	Schema string `json:"schema"`
}

type FilterFunc func(meta Meta, raw json.RawMessage) json.RawMessage

func PackagesAndBundles(meta Meta, raw json.RawMessage) json.RawMessage {
	if meta.Schema == "olm.package" || meta.Schema == "olm.bundle" {
		return raw
	}
	return nil
}

func All(meta Meta, raw json.RawMessage) json.RawMessage {
	return raw
}

func Query(query *gojq.Query) FilterFunc {
	return func(meta Meta, raw json.RawMessage) json.RawMessage {
		out := json.RawMessage{}
		var parsed map[string]interface{}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			logrus.Error(err)
			return out
		}
		iter := query.Run(parsed)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				logrus.Error(err)
			}
			bytes, err := json.Marshal(v)
			if err != nil {
				logrus.Error(err)
			} else {
				out = append(out, bytes...)
			}
		}
		return out
	}
}

func streamBlobs(dir string, filter FilterFunc, blobs chan<- []byte, errs chan<- error, done chan<- struct{}) {

	walkErr := godirwalk.Walk(dir, &godirwalk.Options{
		Unsorted: true,
		Callback: func(path string, de *godirwalk.Dirent) error {
			f, err := os.Open(path)
			if err != nil {
				errs <- fmt.Errorf("error opening %q: %v", path, err)
			}
			decoder := yaml.NewYAMLOrJSONDecoder(f, 1024)
			for {
				doc := json.RawMessage{}
				if err := decoder.Decode(&doc); err != nil {
					break
				}
				var in Meta
				if err := json.Unmarshal(doc, &in); err != nil {
					// Ignore blobs if they are not parsable as meta objects.
					continue
				}
				doc = append(doc, []byte("\n")[0])
				blobs <- filter(in, doc)
			}
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			errs <- fmt.Errorf("skipping %q: %v", osPathname, err)
			return godirwalk.SkipNode
		},
	})
	if walkErr != nil {
		errs <- fmt.Errorf("error walking %q: %v", dir, walkErr)
	}
	done <- struct{}{}
	return
}
