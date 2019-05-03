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
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	radix "github.com/hashicorp/go-immutable-radix"
	"github.com/spf13/afero"
)

var filepathSeparator = string(filepath.Separator)

// A RootMappingFs maps several roots into one. Note that the root of this filesystem
// is directories only, and they will be returned in Readdir and Readdirnames
// in the order given.
type RootMappingFs struct {
	afero.Fs
	rootMapToReal *radix.Node
	virtualRoots  []RootMapping
}

type rootMappingFile struct {
	afero.File
	fs   *RootMappingFs
	name string
	rm   RootMapping
}

type RootMapping struct {
	From string
	To   string

	// Metadata
	Lang string
}

func (r RootMapping) rootKey() string {
	if true || r.Lang == "" {
		return r.From
	}

	// The same root can be mapped from several languages.
	return path.Join(r.From, "___h"+r.Lang)
}

func (r RootMapping) filename(name string) string {
	return filepath.Join(r.To, strings.TrimPrefix(name, r.From))
}

func (rm *RootMapping) clean() {
	rm.From = filepath.Clean(rm.From)
	rm.To = filepath.Clean(rm.To)
}

// NewRootMappingFs creates a new RootMappingFs on top of the provided with
// of root mappings with some optional metadata about the root.
// Note that From represents a virtual root that maps to the actual filename in To.
func NewRootMappingFs(fs afero.Fs, rms ...RootMapping) (*RootMappingFs, error) {
	rootMapToReal := radix.New().Txn()

	for _, rm := range rms {
		(&rm).clean()
		key := []byte(rm.rootKey())
		var mappings []RootMapping
		v, found := rootMapToReal.Get(key)
		if found {
			// There may be more than one language pointing to the same root.
			mappings = v.([]RootMapping)
		}
		mappings = append(mappings, rm)
		rootMapToReal.Insert(key, mappings)
	}

	if rfs, ok := fs.(*afero.BasePathFs); ok {
		fs = NewBasePathRealFilenameFs(rfs)
	}

	rfs := &RootMappingFs{Fs: fs,
		virtualRoots:  rms,
		rootMapToReal: rootMapToReal.Commit().Root()}

	return rfs, nil
}

// NewRootMappingFsFromFromTo is a convenicence variant of NewRootMappingFs taking
// From and To as string pairs.
func NewRootMappingFsFromFromTo(fs afero.Fs, fromTo ...string) (*RootMappingFs, error) {
	rms := make([]RootMapping, len(fromTo)/2)
	for i, j := 0, 0; j < len(fromTo); i, j = i+1, j+2 {
		rms[i] = RootMapping{
			From: fromTo[j],
			To:   fromTo[j+1],
		}

	}

	return NewRootMappingFs(fs, rms...)
}

// TODO(bep) mod do a stat to check for each
func (fs *RootMappingFs) Dirs(name string) ([]FileMeta, error) {
	roots := fs.getRoots(name)
	if roots == nil {
		return nil, nil
	}

	ms := make([]FileMeta, len(roots))
	for i, r := range roots {
		ms[i] = FileMeta{
			metaKeyFilename: r.filename(name),
			metaKeyLang:     r.Lang,
		}
	}

	return ms, nil
}

// Stat returns the os.FileInfo structure describing a given file.  If there is
// an error, it will be of type *os.PathError.
func (fs *RootMappingFs) Stat(name string) (os.FileInfo, error) {
	if fs.isRoot(name) {
		return newDirNameOnlyFileInfo(name), nil
	}

	root, err := fs.getRoot(name)
	if err != nil {
		return nil, err
	}

	filename := root.filename(name)

	fi, err := fs.Fs.Stat(filename)
	if err != nil {
		return nil, err
	}

	// TODO(bep) mod root
	return decorateFileInfo(fs.Fs, nil, fi, filename, "", root.Lang), nil

}

// LstatIfPossible returns the os.FileInfo structure describing a given file.
// It attempts to use Lstat if supported or defers to the os.  In addition to
// the FileInfo, a boolean is returned telling whether Lstat was called.
func (fs *RootMappingFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {

	if fs.isRoot(name) {
		return newDirNameOnlyFileInfo(name), false, nil
	}

	root, err := fs.getRoot(name)
	if err != nil {
		return nil, false, err
	}

	filename := root.filename(name)

	var b bool
	var fi os.FileInfo

	if ls, ok := fs.Fs.(afero.Lstater); ok {
		fi, b, err = ls.LstatIfPossible(filename)
		if err != nil {
			return nil, b, err
		}

	} else {
		fi, err = fs.Stat(filename)
		if err != nil {
			return nil, b, err
		}
	}
	return decorateFileInfo(fs.Fs, nil, fi, filename, "", root.Lang), b, nil
}

func (fs *RootMappingFs) isRoot(name string) bool {
	return name == "" || name == filepathSeparator

}

// Open opens the named file for reading.
func (fs *RootMappingFs) Open(name string) (afero.File, error) {
	if fs.isRoot(name) {
		return &rootMappingFile{name: name, fs: fs}, nil
	}
	root, err := fs.getRoot(name)
	if err != nil {
		return nil, err
	}
	filename := root.filename(name)

	f, err := fs.Fs.Open(filename)
	if err != nil {
		return nil, err
	}
	return &rootMappingFile{File: f, name: name, fs: fs, rm: root}, nil
}

func (fs *RootMappingFs) getRoot(name string) (RootMapping, error) {
	roots := fs.getRoots(name)
	if len(roots) == 0 {
		return RootMapping{}, os.ErrNotExist
	}
	if len(roots) > 1 {
		return RootMapping{}, errors.Errorf("got %d matches", len(roots))
	}

	return roots[0], nil
}

func (fs *RootMappingFs) getRoots(name string) []RootMapping {
	nameb := []byte(filepath.Clean(name))
	_, v, found := fs.rootMapToReal.LongestPrefix(nameb)
	if !found {
		return nil
	}

	return v.([]RootMapping)
}

func (f *rootMappingFile) Readdir(count int) ([]os.FileInfo, error) {
	if f.File == nil {
		dirsn := make([]os.FileInfo, 0)
		for i := 0; i < len(f.fs.virtualRoots); i++ {
			if count != -1 && i >= count {
				break
			}
			rm := f.fs.virtualRoots[i]
			fi := newDirNameOnlyFileInfo(rm.From)
			if rm.Lang != "" {
				fi = NewFileMetaInfo(fi, FileMeta{metaKeyLang: rm.Lang})
			}
			dirsn = append(dirsn, fi)
		}
		return dirsn, nil
	}

	fis, err := f.File.Readdir(count)
	if err != nil {
		return nil, err
	}

	for i, fi := range fis {
		fis[i] = decorateFileInfo(f.fs.Fs, nil, fi, "", filepath.Join(f.Name(), fi.Name()), f.rm.Lang)
	}

	return fis, nil

}

func (f *rootMappingFile) Readdirnames(count int) ([]string, error) {
	dirs, err := f.Readdir(count)
	if err != nil {
		return nil, err
	}
	dirss := make([]string, len(dirs))
	for i, d := range dirs {
		dirss[i] = d.Name()
	}
	return dirss, nil
}

func (f *rootMappingFile) Name() string {
	return f.name
}

func (f *rootMappingFile) Close() error {
	if f.File == nil {
		return nil
	}
	return f.File.Close()
}
