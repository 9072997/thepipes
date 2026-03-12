package cleanfs

import (
	"io/fs"
	"strings"
)

// cleanFS wraps an fs.FS with two transformations:
//  1. Auto-strip nested single-directory prefixes (e.g. embed's "payload/" dir).
//  2. Case-insensitive Open via a pre-built index of files and directories.
type cleanFS struct {
	inner fs.FS
	index map[string]string // lowercase path -> actual path
}

// New returns an fs.FS that strips single-directory prefixes and performs
// case-insensitive lookups against the given filesystem.
// If fsys is already a *cleanFS, it is returned as-is (idempotent).
func New(fsys fs.FS) fs.FS {
	if _, ok := fsys.(*cleanFS); ok {
		return fsys
	}

	// Strip nested single-directory prefixes.
	// Walk from root: if a directory contains exactly one entry and that
	// entry is a directory, descend. Repeat until the directory has
	// multiple entries or contains files.
	for {
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			break
		}
		if len(entries) != 1 || !entries[0].IsDir() {
			break
		}
		sub, err := fs.Sub(fsys, entries[0].Name())
		if err != nil {
			break
		}
		fsys = sub
	}

	// Build case-insensitive index of both files and directories.
	idx := make(map[string]string)
	idx["."] = "." // root
	_ = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't stat
		}
		if path != "." {
			idx[strings.ToLower(path)] = path
		}
		return nil
	})

	return &cleanFS{inner: fsys, index: idx}
}

// Open implements fs.FS. It folds the requested name to lowercase,
// looks it up in the index, and opens the real path from the inner FS.
func (f *cleanFS) Open(name string) (fs.File, error) {
	actual, ok := f.index[strings.ToLower(name)]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return f.inner.Open(actual)
}
