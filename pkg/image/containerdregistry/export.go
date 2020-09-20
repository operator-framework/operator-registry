package containerdregistry

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/images/archive"
	"github.com/operator-framework/operator-registry/pkg/image"
)

// Export exports the given image ref into an oci bundle
// Export does not unpack the root filesystem of the bundle
func (r *Registry) Export(ctx context.Context, ref image.Reference, out string) error {
	err := os.MkdirAll(out, 0700)
	if err != nil {
		return fmt.Errorf("error creating parent directory %s: %v", out, err)
	}

	ctx = ensureNamespace(ctx)

	buf := new(bytes.Buffer)

	err = archive.Export(ctx, r.Store.Content(), buf, archive.WithPlatform(r.platform), archive.WithImage(r.Store.Images(), ref.String()))
	if err != nil {
		return fmt.Errorf("error exporting image %s to oci format: %v", ref, err)
	}

	tarContent := tar.NewReader(buf)

	for {
		hdr, err := tarContent.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Error reading tar file: %v", err)
		}
		if hdr == nil {
			continue
		}
		if err != nil {
			return fmt.Errorf("Invalid entry in image %s: %v", ref, err)
		}
		dstPath := filepath.Join(out, filepath.Clean(hdr.Name))

		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			raw := make([]byte, hdr.Size)
			n, err := tarContent.Read(raw)
			if (err != nil && err != io.EOF) || int64(n) != hdr.Size {
				r.log.Warnf("Incomplete read(%d/%d) %s: %v\n", n, hdr.Size, hdr.Name, err)
				continue
			}

			dir := filepath.Dir(dstPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("error creating directory %s: %v", dir, err)
			}

			f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				return fmt.Errorf("error creating file %s: %v", hdr.Name, err)
			}
			n, err = f.Write(raw)
			io.Copy(f, tarContent)
			if (err != nil && err != io.EOF) || int64(n) != hdr.Size {
				f.Close()
				return fmt.Errorf("error copying file %s, (wrote %d of %d bytes) : %v", hdr.Name, n, hdr.Size, err)
			}
			f.Close()
		case tar.TypeDir:
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return fmt.Errorf("error creating directory %s: %v", hdr.Name, err)
			}

		default:
			return fmt.Errorf("error processing file %s: unknown typeflag '\\x%x'", hdr.Name, hdr.Typeflag)
		}
	}
	return nil
}
