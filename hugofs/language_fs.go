// Copyright 2019 The Hugo Authors. All rights reserved.
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
	"syscall"
	"time"

	"github.com/pkg/errors"

	"github.com/spf13/afero"
)

var (
	_ afero.Fs      = (*LanguageFs)(nil)
	_ afero.Lstater = (*LanguageFs)(nil)
	_ afero.File    = (*LingoDir)(nil)
)

type _LangFilePather interface {
	LanguageAnnouncer
	_FilePather
}

// LanguageAnnouncer is aware of its language.
// TODO(bep) remove?
type LanguageAnnouncer interface {
	Lang() string
	TranslationBaseName() string
}

// FilePather is aware of its file's location.
// TODO(bep) remove
type _FilePather interface {
	// Filename gets the full path and filename to the file.
	Filename() string

	// Path gets the content relative path including file name and extension.
	// The directory is relative to the content root where "content" is a broad term.
	Path() string

	// RealName is FileInfo.Name in its original form.
	RealName() string

	BaseDir() string
}

func NewLanguageFs(langs map[string]bool, sources ...LangFsProvider) (*LanguageFs, error) {
	if len(sources) == 0 {
		return nil, errors.New("requires at least 1 filesystem")
	}

	first := sources[0]

	common := &languageFsCommon{
		languages: langs,
	}

	root := &LanguageFs{languageFsCommon: common, source: first}
	root.root = root

	if len(sources) == 1 {
		return root, nil
	}

	rest := sources[1:]

	parent := root
	for _, fs := range rest {
		lfs := &LanguageFs{languageFsCommon: common, source: fs, root: root}
		parent.child = lfs
		parent = lfs

	}

	return root, nil
}

type FileOpener interface {
	Open() (afero.File, error)
}

/*

Base top/botton

sv/foo/index.md, bar.sv.txt, sar.en.txt
en/foo/index.md, bar.sv.txt, sar.sv.txt
no/foo/index.md
en/images/image.jpg
no/images/image.jpg

foo.ReadDir => 6 files ? Name "no/

or:

sv/foo.ReadDir index.md bar.sv.txt (sv) sar.sv.txt (en)
en/foo.ReadDir index.md  sar.en.txt (sv)


*/

type LangFsProvider interface {
	Fs() afero.Fs
	Lang() string
}

// TODO(bep) mod dir files same name different languages
type LingoDir struct {
	fs      *LanguageFs
	fi      os.FileInfo // TODO(bep) mod remove
	dirname string
}

func (f *LingoDir) Close() error {
	return nil
}

func (f *LingoDir) Name() string {
	panic("not implemented")
}

func (f *LingoDir) Read(p []byte) (n int, err error) {
	panic("not implemented")
}

func (f *LingoDir) ReadAt(p []byte, off int64) (n int, err error) {
	panic("not implemented")
}

func (f *LingoDir) Readdir(count int) ([]os.FileInfo, error) {
	return f.fs.readDirs(f.dirname, count)
}

func (f *LingoDir) Readdirnames(count int) ([]string, error) {
	dirsi, err := f.Readdir(count)
	if err != nil {
		return nil, err
	}

	dirs := make([]string, len(dirsi))
	for i, d := range dirsi {
		dirs[i] = d.Name()
	}
	return dirs, nil
}

func (f *LingoDir) Seek(offset int64, whence int) (int64, error) {
	panic("not implemented")
}

func (f *LingoDir) Stat() (os.FileInfo, error) {
	panic("not implemented")
}

func (f *LingoDir) Sync() error {
	panic("not implemented")
}

func (f *LingoDir) Truncate(size int64) error {
	panic("not implemented")
}

func (f *LingoDir) Write(p []byte) (n int, err error) {
	panic("not implemented")
}

func (f *LingoDir) WriteAt(p []byte, off int64) (n int, err error) {
	panic("not implemented")
}

func (f *LingoDir) WriteString(s string) (ret int, err error) {
	panic("not implemented")
}

type LanguageFs struct {
	*languageFsCommon
	root   *LanguageFs
	child  *LanguageFs
	source LangFsProvider
}

func (fs *LanguageFs) Chmod(n string, m os.FileMode) error {
	return syscall.EPERM
}

func (fs *LanguageFs) Chtimes(n string, a, m time.Time) error {
	return syscall.EPERM
}

// TODO(bep) mod lstat
func (fs *LanguageFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	fi, _, err := fs.pickFirst(name)
	if err != nil {
		return nil, false, err
	}
	if fi.IsDir() {
		return decorateFileInfo(fs, fs.getOpener(name), fi, "", "", ""), false, nil
	}

	return nil, false, errors.Errorf("lstat: files not supported: %q", name)

}

func (fs *LanguageFs) Mkdir(n string, p os.FileMode) error {
	return syscall.EPERM
}

func (fs *LanguageFs) MkdirAll(n string, p os.FileMode) error {
	return syscall.EPERM
}

func (fs *LanguageFs) Name() string {
	return "WeightedFileSystem"
}

func (fs *LanguageFs) Open(name string) (afero.File, error) {
	fi, lfs, err := fs.pickFirst(name)
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		panic("currently only dirs in here")
	}

	return &LingoDir{
		fs:      lfs,
		fi:      fi,
		dirname: name,
	}, nil

}

func (fs *LanguageFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	panic("not implemented")
}

func (fs *LanguageFs) ReadDir(name string) ([]os.FileInfo, error) {
	panic("not implemented")
}

func (fs *LanguageFs) Remove(n string) error {
	return syscall.EPERM
}

func (fs *LanguageFs) RemoveAll(p string) error {
	return syscall.EPERM
}

func (fs *LanguageFs) Rename(o, n string) error {
	return syscall.EPERM
}

func (fs *LanguageFs) Stat(name string) (os.FileInfo, error) {
	fi, _, err := fs.LstatIfPossible(name)
	return fi, err
}

func (fs *LanguageFs) Create(n string) (afero.File, error) {
	return nil, syscall.EPERM
}

func (fs *LanguageFs) getOpener(name string) func() (afero.File, error) {
	return func() (afero.File, error) {
		return fs.Open(name)
	}
}

func (fs *LanguageFs) applyMeta(name string, fis []os.FileInfo) []os.FileInfo {
	fisn := make([]os.FileInfo, len(fis))
	for i, fi := range fis {
		if fi.IsDir() {
			fisn[i] = decorateFileInfo(fs.root, fs.root.getOpener(filepath.Join(name, fi.Name())), fi, "", "", "")
			continue
		}

		lang := fs.source.Lang()
		fileLang, translationBaseName := fs.langInfoFrom(fi.Name())
		weight := 0
		if fileLang != "" {
			weight = 1
			if fileLang == lang {
				// Give priority to myfile.sv.txt inside the sv filesystem.
				weight++
			}
			lang = fileLang
		}

		// TODO(bep) mod path++

		fisn[i] = NewFileMetaInfo(fi, FileMeta{
			metaKeyLang:           lang,
			"weight":              weight,
			"translationBaseName": translationBaseName,
		})

	}

	return fisn
}

func (fs *LanguageFs) collectFileInfos(root *LanguageFs, name string) ([]os.FileInfo, error) {
	var fis []os.FileInfo
	current := root
	for current != nil {
		fi, err := current.source.Fs().Stat(name)
		if err == nil {
			// Gotta match!
			fis = append(fis, fi)
		} else if !os.IsNotExist(err) {
			// Real error
			return nil, err
		}

		// Continue
		current = current.child

	}

	return fis, nil
}

func (fs *LanguageFs) filterDuplicates(fis []os.FileInfo) []os.FileInfo {
	type idxWeight struct {
		idx    int
		weight int
	}

	keep := make(map[string]idxWeight)

	for i, fi := range fis {
		if fi.IsDir() {
			continue
		}
		meta := fi.(FileMetaInfo).Meta()
		weight := meta.GetInt("weight")
		if weight > 0 {
			name := fi.Name()
			k, found := keep[name]
			if !found || weight > k.weight {
				keep[name] = idxWeight{
					idx:    i,
					weight: weight,
				}
			}
		}
	}

	if len(keep) > 0 {
		toRemove := make(map[int]bool)
		for i, fi := range fis {
			if fi.IsDir() {
				continue
			}
			k, found := keep[fi.Name()]
			if found && i != k.idx {
				toRemove[i] = true
			}
		}

		filtered := fis[:0]
		for i, fi := range fis {
			if !toRemove[i] {
				filtered = append(filtered, fi)
			}
		}
		fis = filtered
	}

	return fis
}

func (fs *LanguageFs) pickFirst(name string) (os.FileInfo, *LanguageFs, error) {
	current := fs
	for current != nil {
		fi, err := current.source.Fs().Stat(name)
		if err == nil {
			// Gotta match!
			return fi, current, nil
		}

		if !os.IsNotExist(err) {
			// Real error
			return nil, nil, err
		}

		// Continue
		current = current.child

	}

	// Not found
	return nil, nil, os.ErrNotExist
}

func (fs *LanguageFs) readDirs(name string, count int) ([]os.FileInfo, error) {

	collect := func(current *LanguageFs) ([]os.FileInfo, error) {
		d, err := current.source.Fs().Open(name)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			return nil, nil
		} else {
			defer d.Close()
			dirs, err := d.Readdir(-1)
			if err != nil {
				return nil, err
			}
			return current.applyMeta(name, dirs), nil
		}
	}

	var dirs []os.FileInfo

	current := fs
	for current != nil {

		fis, err := collect(current)
		if err != nil {
			return nil, err
		}

		dirs = append(dirs, fis...)
		if count > 0 && len(dirs) >= count {
			return dirs[:count], nil
		}

		current = current.child

	}

	return fs.filterDuplicates(dirs), nil

}

func NewLangFsProvider(lang string, fs afero.Fs) LangFsProvider {
	return langFs{fs: fs, lang: lang}
}

// langFs wraps a afero.Fs with language information.
type langFs struct {
	fs   afero.Fs
	lang string
}

func (m langFs) Fs() afero.Fs {
	return m.fs
}

func (m langFs) Lang() string {
	return m.lang
}

type fileOpener struct {
	os.FileInfo
	openFileFunc
}

// TODO(bep) mod names, names, names
//  TODO(bep) mod remove me
type lingoFileInfo struct {
	os.FileInfo

	lang                string
	translationBaseName string

	filename string // the real filename in the source filesystem
	baseDir  string
	path     string

	openFileFunc

	// Set when there is language information in the filename.
	weight int
}

func (fi lingoFileInfo) BaseDir() string {
	return fi.baseDir
}

func (fi lingoFileInfo) Filename() string {
	return fi.filename
}

func (fi lingoFileInfo) Lang() string {
	return fi.lang
}

func (fi lingoFileInfo) Path() string {
	return fi.path
}

func (fi lingoFileInfo) RealName() string {
	panic("remove me")
}

// TranslationBaseName returns the base filename without any language
// or file extension.
// E.g. myarticle.en.md becomes myarticle.
func (fi lingoFileInfo) TranslationBaseName() string {
	return fi.translationBaseName
}

type languageFsCommon struct {
	languages map[string]bool
}

// Try to extract the language from the given filename.
// Any valid language identificator in the name will win over the
// language set on the file system, e.g. "mypost.en.md".
func (l *languageFsCommon) langInfoFrom(name string) (string, string) {
	var lang string

	baseName := filepath.Base(name)
	ext := filepath.Ext(baseName)
	translationBaseName := baseName

	if ext != "" {
		translationBaseName = strings.TrimSuffix(translationBaseName, ext)
	}

	fileLangExt := filepath.Ext(translationBaseName)
	fileLang := strings.TrimPrefix(fileLangExt, ".")

	if l.languages[fileLang] {
		lang = fileLang
		translationBaseName = strings.TrimSuffix(translationBaseName, fileLangExt)
	}

	return lang, translationBaseName

}

type openFileFunc func() (afero.File, error)

func (f openFileFunc) Open() (afero.File, error) {
	return f()
}

func decorateFileInfo(
	fs afero.Fs,
	opener func() (afero.File, error),
	fi os.FileInfo,
	filename,
	path,
	lang string) os.FileInfo {

	var m FileMeta

	if fim, ok := fi.(FileMetaInfo); ok {
		m = fim.Meta()
		if fn, ok := m[metaKeyFilename]; ok {
			filename = fn.(string)
		}
	} else {
		m = make(FileMeta)
		fim := NewFileMetaInfo(fi, m)
		fi = fim

	}

	if opener != nil {
		m.setIfNotZero(metaKeyOpener, opener)
	}

	if fi.IsDir() && fs != nil { // TODO(bep) mod check + argument
		m.setIfNotZero(metaKeyFs, fs)
	}

	m.setIfNotZero(metaKeyLang, lang)
	m.setIfNotZero(metaKeyPath, path)
	m.setIfNotZero(metaKeyFilename, filename)

	return fi

}
