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

		if hdr.Typeflag == tar.TypeDir {
			return errors.Errorf("was not expecting directory entries in tar (got %s)", hdr.Name)
		} else if hdr.Typeflag != tar.TypeReg {
			return errors.Errorf("found non-regular file entry in tar (type: %v) file: %s", hdr.Typeflag, hdr.Name)
		}
		fn := hdr.Name
		fpath := filepath.Join(dir, filepath.FromSlash(fn))
		if strings.HasSuffix(fn, constants.WhiteoutDeleteSuffix) {
			if err := os.RemoveAll(strings.TrimSuffix(fpath, constants.WhiteoutDeleteSuffix)); err != nil {
				return errors.Wrapf(err, "failed to realize delete whiteout file %s", fn)
			}
			continue
		}
		if strings.HasSuffix(fn, constants.WhiteoutPlaceholderSuffix) {
			mode := os.FileMode(os.ModeDir | hdr.FileInfo().Mode())
			if err := os.Mkdir(strings.TrimSuffix(fpath, constants.WhiteoutPlaceholderSuffix), mode); err != nil {
				return errors.Wrapf(err, "failed to realize empty directory placeholder whiteout for dir %s", fn)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), hdr.FileInfo().Mode()&os.ModeDir); err != nil {
			// TODO(ahmetb) bad idea to decide on chmod of the dir here! how come tar allows dir entries but you can't add them while compressing?
			return errors.Wrapf(err, "failed to ensure parent dirs created for  tar entry %s", fn)
		}

		f, err := os.OpenFile(fpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, hdr.FileInfo().Mode())
		if err != nil {
			return errors.Wrapf(err, "failed to create file for tar entry %s", fn)
		}

		// copy over contents
		if _, err := io.Copy(f, tr); err != nil {
			return errors.Wrapf(err, "failed to copy file contents for tar entry %s", fn)
		}
		if err := f.Close(); err != nil {
			return errors.Wrapf(err, "failed to close copied file for tar entry %s", fn)
		}
		if err := os.Chtimes(fpath, hdr.ModTime, hdr.ModTime); err != nil {
			return errors.Wrapf(err, "failed to change times of copied file for tar entry %s", fn)
		}
	}
	return nil
}
