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
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gohugoio/hugo/langs"
	"github.com/spf13/viper"

	"github.com/spf13/afero"

	"github.com/stretchr/testify/require"
)

// TODO(bep) mod
// tc-lib-color/class-Com.Tecnick.Color.Css and class-Com.Tecnick.Color.sv.Css
func TestLanguageFs(t *testing.T) {
	assert := require.New(t)
	v := viper.New()
	v.Set("contentDir", "content")

	langSet := langs.Languages{
		langs.NewLanguage("en", v),
		langs.NewLanguage("sv", v),
	}.AsSet()

	enFs := langFs{lang: "en", fs: afero.NewMemMapFs()}
	svFs := langFs{lang: "sv", fs: afero.NewMemMapFs()}

	for _, fs := range []langFs{enFs, svFs} {
		assert.NoError(afero.WriteFile(fs.Fs(), filepath.FromSlash("blog/a.txt"), []byte("abc"), 0777))

		for _, fs2 := range []langFs{enFs, svFs} {
			lingoName := fmt.Sprintf("lingo.%s.txt", fs2.lang)
			assert.NoError(afero.WriteFile(fs.Fs(), filepath.FromSlash("blog/"+lingoName), []byte(lingoName), 0777))
		}

	}

	lfs, err := NewLanguageFs(langSet, enFs, svFs)
	assert.NoError(err)

	blog, err := lfs.Open("blog")
	assert.NoError(err)

	names, err := blog.Readdirnames(-1)
	assert.NoError(err)
	assert.Equal(4, len(names), names)
	assert.Equal([]string{"a.txt", "lingo.en.txt", "a.txt", "lingo.sv.txt"}, names)

	fis, err := blog.Readdir(-1)
	assert.NoError(err)
	assert.Equal(4, len(fis))

	enFim := fis[0].(FileMetaInfo).Meta()
	svFim := fis[2].(FileMetaInfo).Meta()

	assert.Equal("en", enFim.Lang())
	assert.Equal("sv", svFim.Lang())

}

/*

theme/a/mysvblogcontent => /blog [sv]
theme/b/myenblogcontent=> /blog [en]

*/

func TestLanguageRootMapping(t *testing.T) {
	assert := require.New(t)
	v := viper.New()
	v.Set("contentDir", "content")

	/*langSet := langs.Languages{
		langs.NewLanguage("en", v),
		langs.NewLanguage("sv", v),
	}.AsSet()*/

	fs := afero.NewMemMapFs()

	testfile := "test.txt"

	assert.NoError(afero.WriteFile(fs, filepath.Join("themes/a/mysvblogcontent", testfile), []byte("some sv blog content"), 0755))
	assert.NoError(afero.WriteFile(fs, filepath.Join("themes/a/myenblogcontent", testfile), []byte("some en blog content in a"), 0755))

	assert.NoError(afero.WriteFile(fs, filepath.Join("themes/a/mysvdocs", testfile), []byte("some sv docs content"), 0755))

	assert.NoError(afero.WriteFile(fs, filepath.Join("themes/b/myenblogcontent", testfile), []byte("some en content"), 0755))

	bfs := NewBasePathRealFilenameFs(afero.NewBasePathFs(fs, "themes").(*afero.BasePathFs))

	rfs, err := NewRootMappingFs(bfs,
		RootMapping{
			From: "blog",
			To:   "a/mysvblogcontent",
			Lang: "sv",
		},
		RootMapping{
			From: "blog",
			To:   "a/myenblogcontent",
			Lang: "en",
		},
		RootMapping{
			From: "docs",
			To:   "a/mysvdocs",
			Lang: "sv",
		},
	)

	assert.NoError(err)

	dirs, err := rfs.Dirs("blog")
	assert.NoError(err)
	assert.Equal(2, len(dirs))

	for _, dir := range dirs {
		fmt.Println(">>> DIR", dir )
	}

}
