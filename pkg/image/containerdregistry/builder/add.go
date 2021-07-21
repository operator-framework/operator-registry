package builder

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)


const whiteoutPrefix = ".wh."
const opaqueWhiteout = ".wh..wh..opq"

func (i *imageBuilder) Add(ctx context.Context, platformMatcher platforms.MatchComparer, srcPath, dstPath string) error {
	ctx = ensureNamespace(ctx)
	action := fmt.Sprintf("ADD %s %s", srcPath, dstPath)
	return i.updateManifest(ctx, platformMatcher, func(m *ocispec.Manifest) (bool, string, error) {
		data, diffID, err := layerFromPath(srcPath, dstPath)
		if err != nil {
			return false, "", fmt.Errorf("error creating layer for %s:%s : %v", srcPath, dstPath, err)
		}
		layerDesc, err := i.descriptorFromBytes(ctx, data, images.MediaTypeDockerSchema2LayerGzip)
		if err != nil {
			return false, "", fmt.Errorf("error creating layer descriptor for %s:%s : %v", srcPath, dstPath, err)
		}

		confBlob, err := content.ReadBlob(ctx, i.registry.Content(), m.Config)
		if err != nil {
			return false, "", err
		}

		config := ocispec.Image{}
		if err := json.Unmarshal(confBlob, &config); err != nil {
			return false, "", fmt.Errorf("failed to get imageConfig from manifest %v", err)
		}
		config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, *diffID)

		if !i.NoHistory {
			historyEntry := ocispec.History{
				CreatedBy:  "opm generate",
				EmptyLayer: false,
				Comment:    action,
			}
			if !i.OmitTimestamp {
				historyEntry.Created = i.WithTimestamp
				if historyEntry.Created == nil {
					ts := time.Now()
					historyEntry.Created = &ts
				}
			}
			if len(config.History) == 0 {
				config.History = []ocispec.History{}
			}
			config.History = append(config.History, historyEntry)
		}

		configDesc, err := i.newDescriptor(ctx, config, images.MediaTypeDockerSchema2Config)
		if err != nil {
			return false, "", err
		}
		m.Config = configDesc
		m.Layers = append(m.Layers, layerDesc)
		return true, action, nil
	})
}

// layerFromPath builds a single tgz image layer from a directory of files
// returns the gzip data in a byte buffer and the digest of the uncompressed files
func layerFromPath(srcPath, dstPath string) ([]byte, *digest.Digest, error) {
	if _, err := os.Stat(srcPath); err != nil {
		return nil, nil, fmt.Errorf("bad source path %s: %v", srcPath, err)
	}

	// set up our layer pipeline
	//
	//                  -> gz -> byte buffer
	//                /
	// files -> tar -
	//                \
	//                  -> sha256 -> digest
	//

	// the byte buffer contains compressed layer data,
	// and the hash is the digest of the uncompressed layer
	// data, which docker requires (oci does not)

	// output writers
	hash := sha256.New()
	var buf bytes.Buffer

	// from gzip to buffer
	gzipWriter := gzip.NewWriter(&buf)

	// from files to hash/gz
	hashAndGzWriter := io.MultiWriter(hash, gzipWriter)
	writer := tar.NewWriter(hashAndGzWriter)

	if srcPath == "" {
		header := tar.Header{
			Typeflag:   tar.TypeReg,
			Name:       filepath.Clean(filepath.Join(filepath.Dir(dstPath), whiteoutPrefix+filepath.Base(dstPath) )),
		}
		err := writer.WriteHeader(&header)
		if err != nil {
			return nil, nil, fmt.Errorf("error writing tar header %v: %v", header, err)
		}
	} else if err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("child error %s: %v", path, err)
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if info.Name() == opaqueWhiteout || strings.HasPrefix(info.Name(), whiteoutPrefix) {
			return fmt.Errorf("cannot add file to layer: illegal prefix '%s' for path %s", whiteoutPrefix, path)
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("error getting tar file header for  %s: %v", info.Name(), err)
		}

		newPath := filepath.Clean(filepath.Join(dstPath, path[len(srcPath):]))
		header.Name = newPath

		err = writer.WriteHeader(header)

		if err != nil {
			return fmt.Errorf("error writing tar header %v: %v", header, err)
		}

		// if it's a directory, just write the header and continue
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening file to add %s: %v", path, err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				logrus.Warnf("error closing file: %s", err.Error())
			}
		}()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("error copying file to add %s: %v", path, err)
		}

		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("error creating layer tar: %v", err)
	}

	// close writer here to get the correct hash - defer will not work
	if err := writer.Close(); err != nil {
		return nil, nil, fmt.Errorf("error closing hashwriter: %v", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, nil, fmt.Errorf("error closing gzipwriter: %v", err)
	}

	b, err := ioutil.ReadAll(&buf)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading tar bytes: %v", err)
	}
	diffID := digest.NewDigestFromBytes(digest.SHA256, hash.Sum(nil))

	return b, &diffID, nil
}
