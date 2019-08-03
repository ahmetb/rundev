package fsutil

import (
	"archive/tar"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ApplyPatch(dir string, r io.ReadCloser) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrap(err, "error reading tar header")
		}

		fn := hdr.Name
		fpath := filepath.Join(dir, filepath.FromSlash(fn))

		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(fpath, hdr.FileInfo().Mode()); err != nil {
				return errors.Wrapf(err, "failed to mkdir for tar dir entry %s", fn)
			}
			continue
		} else if hdr.Typeflag != tar.TypeReg {
			return errors.Errorf("found non-regular file entry in tar (type: %v) file: %s", hdr.Typeflag, hdr.Name)
		}

		if strings.HasSuffix(fn, constants.WhiteoutDeleteSuffix) {
			if err := os.RemoveAll(strings.TrimSuffix(fpath, constants.WhiteoutDeleteSuffix)); err != nil {
				return errors.Wrapf(err, "failed to realize delete whiteout file %s", fn)
			}
			continue
		}

		// copy regular file
		f, err := os.OpenFile(fpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, hdr.FileInfo().Mode())
		if err != nil {
			return errors.Wrapf(err, "failed to create file for tar entry %s", fn)
		}
		if _, err := io.Copy(f, tr); err != nil {
			return errors.Wrapf(err, "failed to copy file contents for tar entry %s", fn)
		}
		if err := f.Close(); err != nil {
			return errors.Wrapf(err, "failed to close copied file for tar entry %s", fn)
		}
		if err := os.Chmod(fpath, hdr.FileInfo().Mode()); err != nil {
			return errors.Wrapf(err, "failed to chmod file for tar entry %s", fn)
		}
		if err := os.Chtimes(fpath, hdr.ModTime, hdr.ModTime); err != nil {
			return errors.Wrapf(err, "failed to change times of copied file for tar entry %s", fn)
		}
	}
	return nil
}
