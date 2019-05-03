// Copyright 2018 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hugofs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// RealFilenameInfo is a thin wrapper around os.FileInfo adding the real filename.
type RealFilenameInfo interface {
	os.FileInfo

	// This is the real filename to the file in the underlying filesystem.
	RealFilename() string
}

// TODO(bep) mod consolidate
type FilePathPather interface {
	Path() string
}

type FileNamer interface {
	Filename() string
}

type LangProvider interface {
	Lang() string
}

// TODO(bep) mod name + consider this vs base fs
// TODO(bep) mod remove all of these
type VirtualFileInfo interface {
	VirtualRoot() string
}

// NewBasePathRealFilenameFs returns a new BasePathRealFilenameFs instance
// using base.
func NewBasePathRealFilenameFs(base *afero.BasePathFs) *BasePathRealFilenameFs {
	basePath, _ := base.RealPath("")
	basePath = strings.TrimLeft(basePath, "."+string(os.PathSeparator))
	return &BasePathRealFilenameFs{BasePathFs: base, basePath: basePath}
}

// BasePathRealFilenameFs is a thin wrapper around afero.BasePathFs that
// provides the real filename in Stat and LstatIfPossible.
type BasePathRealFilenameFs struct {
	*afero.BasePathFs
	basePath string
}

// Stat returns the os.FileInfo structure describing a given file.  If there is
// an error, it will be of type *os.PathError.
func (b *BasePathRealFilenameFs) Stat(name string) (os.FileInfo, error) {
	fi, err := b.BasePathFs.Stat(name)
	if err != nil {
		return nil, err
	}

	if _, ok := fi.(RealFilenameInfo); ok {
		panic("TODO(bep) mod")
	}

	filename, err := b.RealPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}

	return decorateFileInfo(b, b.getOpener(name), fi, filename, "", ""), nil

}

func (b *BasePathRealFilenameFs) getOpener(name string) func() (afero.File, error) {
	return func() (afero.File, error) {
		return b.Open(name)
	}
}

// LstatIfPossible returns the os.FileInfo structure describing a given file.
// It attempts to use Lstat if supported or defers to the os.  In addition to
// the FileInfo, a boolean is returned telling whether Lstat was called.
func (b *BasePathRealFilenameFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {

	fi, ok, err := b.BasePathFs.LstatIfPossible(name)
	if err != nil {
		return nil, false, err
	}

	if hasAllFileMetaKeys(fi, metaKeyFilename) {
		return fi, ok, nil
	}

	filename, err := b.RealPath(name)
	if err != nil {
		return nil, false, &os.PathError{Op: "lstat", Path: name, Err: err}
	}

	return decorateFileInfo(b, b.getOpener(name), fi, filename, "", ""), ok, nil
}

// Open opens the named file for reading.
func (fs *BasePathRealFilenameFs) Open(name string) (afero.File, error) {
	f, err := fs.BasePathFs.Open(name)

	if err != nil {
		return nil, err
	}
	return &realFilenameFile{File: f, fs: fs}, nil
}

type realFilenameFile struct {
	afero.File
	fs *BasePathRealFilenameFs
}

// Readdir creates FileInfo entries by calling Lstat if possible.
func (l *realFilenameFile) Readdir(c int) (ofi []os.FileInfo, err error) {
	names, err := l.File.Readdirnames(c)
	if err != nil {
		return nil, err
	}

	fis := make([]os.FileInfo, len(names))

	for i, name := range names {
		fi, _, err := l.fs.LstatIfPossible(filepath.Join(l.Name(), name))

		if err != nil {
			return nil, err
		}
		fis[i] = fi
	}

	return fis, err
}
