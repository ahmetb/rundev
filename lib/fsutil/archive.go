package fsutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// PatchArchive creates a tarball for given operations in baseDir and returns its size.
func PatchArchive(baseDir string, ops []DiffOp) (io.Reader, int, error) {
	var b bytes.Buffer
	gw, err := gzip.NewWriterLevel(&b, gzip.BestSpeed)
	if err != nil {
		return nil, -1, errors.Wrap(err, "failed to initialize gzip writer")
	}
	tw := tar.NewWriter(gw)

	files, err := normalizeFiles(baseDir, ops)
	if err != nil {
		return nil, -1, errors.Wrap(err, "failed to normalize file list")
	}

	for _, v := range files {
		if err := addFile(tw, v); err != nil {
			return nil, -1, errors.Wrap(err, "tar failure")
		}
	}
	if err := tw.Close(); err != nil {
		return nil, -1, errors.Wrap(err, "failed to finalize tarball writer")
	}
	if err := gw.Close(); err != nil {
		return nil, -1, errors.Wrap(err, "failed to finalize gzip writer")
	}
	return &b, b.Len(), nil
}

func addFile(tw *tar.Writer, file archiveFile) error {
	if file.stat.Mode()&os.ModeSymlink != 0 {
		return errors.Errorf("adding symlinks currently not supported, file:%s", file.fullPath)
	}
	hdr, err := tar.FileInfoHeader(file.stat, "")
	if err != nil {
		return errors.Wrapf(err, "failed to create tar header for file %s", file.fullPath)
	}
	hdr.Name = filepath.ToSlash(file.extractPath) // tar paths must be forward slash
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, "failed to write tar header")
	}
	if file.stat.Size() == 0 {
		return nil
	}
	f, err := os.Open(file.fullPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open file %s for tar-ing", file.fullPath)
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return errors.Wrapf(err, "failed to copy file %s into tar", hdr.Name)
}

type archiveFile struct {
	fullPath    string
	extractPath string
	stat        os.FileInfo
}

// normalizeFiles returns all list of files that should be added to the archive
// by creating whiteout files (indicating deletions, and empty dir placeholders),
// and recursively traversing directories to be added.
func normalizeFiles(baseDir string, ops []DiffOp) ([]archiveFile, error) {
	var out []archiveFile
	for _, op := range ops {
		fullPath := filepath.Join(baseDir, filepath.FromSlash(op.Path))
		if op.Type == DiffOpDel {
			// create a whiteout file
			out = append(out, archiveFile{
				fullPath:    fullPath,
				extractPath: op.Path + constants.WhiteoutDeleteSuffix,
				stat:        whiteoutStat{name: filepath.Base(fullPath)},
			})
		} else if op.Type == DiffOpAdd {
			fi, err := os.Stat(fullPath)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to stat file %s for tar-ing", fullPath)
			}

			if !fi.IsDir() {
				out = append(out, archiveFile{
					fullPath:    fullPath,
					extractPath: op.Path,
					stat:        nanosecMaskingStat{fi},
				})
			} else {
				// directories must be traversed recursively
				files, err := expandDirEntries(fullPath)
				if err != nil {
					return nil, err
				}
				for _, f := range files {
					extractPath, err := filepath.Rel(baseDir, f.fullPath)
					if err != nil {
						return nil, errors.Wrapf(err, "failed to calculate relative path (%s and %s)", baseDir, f.fullPath)
					}
					out = append(out, archiveFile{
						fullPath:    f.fullPath,
						extractPath: extractPath,
						stat:        nanosecMaskingStat{f.stat},
					})
				}
			}
		} else {
			return nil, errors.Errorf("unknown diff operation type (%v)", op.Type)
		}
	}
	return out, nil
}

type tarEntry struct {
	fullPath string
	stat     os.FileInfo
}

// walkDir walks dir recursively to list directory end file entries in sorted order.
func expandDirEntries(dir string) ([]tarEntry, error) {
	var out []tarEntry
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read info for dir %s", dir)
	}
	ls, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read dir %s", dir)
	}

	// add self (dir entry)
	out = append(out, tarEntry{dir, zeroSizeStat{stat}})
	// add child entries
	for _, fi := range ls {
		fp := filepath.Join(dir, fi.Name())
		if !fi.IsDir() {
			v := tarEntry{fp, fi}
			out = append(out, v)
		} else {
			entries, err := expandDirEntries(fp)
			if err != nil {
				return nil, err
			}
			out = append(out, entries...)
		}
	}
	return out, nil
}

type countWriter struct{ n int }

func (c *countWriter) Write(p []byte) (n int, err error) {
	c.n += len(p)
	return len(p), err
}
