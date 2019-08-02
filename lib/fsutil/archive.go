package fsutil

import (
	"archive/tar"
	"bytes"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// PatchArchive creates a tarball for given operations in baseDir.
func PatchArchive(baseDir string, ops []DiffOp) (io.Reader, error) {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	files, err := normalizeFiles(baseDir, ops)
	if err != nil {
		return nil, errors.Wrap(err, "failed to normalize file list")
	}

	for _, v := range files {
		if err := addFile(tw, v); err != nil {
			return nil, errors.Wrap(err, "tar failure")
		}
	}
	if err := tw.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to finalize tarball")
	}
	return &b, nil
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
		fi, err := os.Stat(fullPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to stat file %s for tar-ing", fullPath)
		}
		if op.Type == DiffOpDel {
			out = append(out, archiveFile{
				fullPath:    fullPath,
				extractPath: op.Path + constants.WhiteoutDeleteSuffix,
				stat:        whiteoutStat{name: filepath.Base(fullPath), sys: fi.Sys()},
			})
		} else if op.Type == DiffOpAdd {
			if !fi.IsDir() {
				out = append(out, archiveFile{
					fullPath:    fullPath,
					extractPath: op.Path,
					stat:        fi,
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
						stat:        f.stat,
					})
				}
			}
		} else {
			return nil, errors.Errorf("unknown diff operation type (%v)", op.Type)
		}
	}
	return out, nil
}

type fileEntry struct {
	fullPath string
	stat     os.FileInfo
}

// walkDir lists all the files recursively in dir and
// returns whiteout file entries for empty directories.
func expandDirEntries(dir string) ([]fileEntry, error) {
	var out []fileEntry
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read info for dir %s", dir)
	}
	ls, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read dir %s", dir)
	}
	if len(ls) == 0 {
		fn := dir + constants.WhiteoutPlaceholderSuffix
		out = append(out, fileEntry{fn, whiteoutStat{name: filepath.Base(dir), sys: stat.Sys()}})
		return out, nil
	}
	for _, fi := range ls {
		fp := filepath.Join(dir, fi.Name())
		if !fi.IsDir() {
			v := fileEntry{fp, fi}
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
