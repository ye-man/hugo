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

// Package hugofs provides the file systems used by Hugo.
package hugofs

import (
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/spf13/cast"

	"github.com/gohugoio/hugo/common/hreflect"

	"github.com/spf13/afero"
)

const (
	metaKeyFilename = "filename"
	metaKeyPath     = "path"
	metaKeyLang     = "lang"
	metaKeyFs       = "fs"
	metaKeyOpener   = "opener"
)

type FileMeta map[string]interface{}

func hasAllFileMetaKeys(fi os.FileInfo, keys ...string) bool {
	fim, ok := fi.(FileMetaInfo)
	if !ok {
		return false
	}

	m := fim.Meta()
	if len(m) == 0 {
		return false
	}

	for _, k := range keys {
		if _, found := m[k]; !found {
			return false
		}
	}

	return true
}

func (f FileMeta) GetInt(key string) int {
	return cast.ToInt(f[key])
}

func (f FileMeta) GetString(key string) string {
	return cast.ToString(f[key])
}

func (f FileMeta) Filename() string {
	return f.stringV(metaKeyFilename)
}

func (f FileMeta) Lang() string {
	return f.stringV(metaKeyLang)
}

func (f FileMeta) Path() string {
	return f.stringV(metaKeyPath)
}

func (f FileMeta) Fs() afero.Fs {
	if v, found := f[metaKeyFs]; found {
		return v.(afero.Fs)
	}
	return nil
}

func (f FileMeta) Open() (afero.File, error) {
	v, found := f[metaKeyOpener]
	if !found {
		return nil, errors.New("file opener not found")
	}
	return v.(func() (afero.File, error))()
}

func (f FileMeta) stringV(key string) string {
	if v, found := f[key]; found {
		return v.(string)
	}
	return ""
}

func (f FileMeta) setIfNotZero(key string, val interface{}) {
	if !hreflect.IsTruthful(val) {
		return
	}
	f[key] = val
}

type FileMetaInfo interface {
	os.FileInfo
	Meta() FileMeta
}

type fileInfoMeta struct {
	os.FileInfo
	m FileMeta
}

func (fi *fileInfoMeta) Meta() FileMeta {
	return fi.m
}

func NewFileMetaInfo(fi os.FileInfo, m FileMeta) os.FileInfo {
	if fim, ok := fi.(FileMetaInfo); ok {
		mergeFileMeta(fim.Meta(), m)
	}
	return &fileInfoMeta{FileInfo: fi, m: m}
}

// Merge metadata, last entry wins.
func mergeFileMeta(from, to FileMeta) {
	for k, v := range from {
		if _, found := to[k]; !found {
			to[k] = v
		}
	}
}
type dirNameOnlyFileInfo struct {
	name string
}

func (fi *dirNameOnlyFileInfo) Name() string {
	return fi.name
}

func (fi *dirNameOnlyFileInfo) Size() int64 {
	panic("not implemented")
}

func (fi *dirNameOnlyFileInfo) Mode() os.FileMode {
	return os.ModeDir
}

func (fi *dirNameOnlyFileInfo) ModTime() time.Time {
	panic("not implemented")
}

func (fi *dirNameOnlyFileInfo) IsDir() bool {
	return true
}

func (fi *dirNameOnlyFileInfo) Sys() interface{} {
	return nil
}

func newDirNameOnlyFileInfo(name string) os.FileInfo {
	return &dirNameOnlyFileInfo{name: name}
}
