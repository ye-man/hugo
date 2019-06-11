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

package hugolib

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gohugoio/hugo/htesting"
	"github.com/gohugoio/hugo/hugofs"

	"github.com/gohugoio/testmodBuilder/mods"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TODO(bep) mod this fails when testmodBuilder is also building ...
func TestHugoModules(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// We both produce and consume these for all of these,
	// a test matrix Keanu Reeves would appreciate.
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, ignoreVendor := range []bool{false, true} {
			testmods := mods.CreateModules(goos).Collect()
			ignoreVendor := ignoreVendor
			for _, m := range testmods {
				m := m
				name := fmt.Sprintf("%s/ignoreVendor=%t", strings.Replace(m.Path(), ".", "/", -1), ignoreVendor)
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					assert := require.New(t)

					v := viper.New()

					workingDir, clean, err := htesting.CreateTempDir(hugofs.Os, "hugo-modules-test")
					assert.NoError(err)
					defer clean()

					configTemplate := `
baseURL = "https://example.com"
title = "My Modular Site"
workingDir = %q
theme = %q
ignoreVendor = %t

`

					config := fmt.Sprintf(configTemplate, workingDir, m.Path(), ignoreVendor)

					b := newTestSitesBuilder(t)

					// Need to use OS fs for this.
					b.Fs = hugofs.NewDefault(v)

					b.WithWorkingDir(workingDir).WithConfigFile("toml", config)
					b.WithContent("page.md", `
---
title: "Foo"
---
`)
					b.WithTemplates("home.html", `

{{ $mod := .Site.Data.modinfo.module }}
Mod Name: {{ $mod.name }}
Mod Version: {{ $mod.version }}
----
{{ range $k, $v := .Site.Data.modinfo }}
- {{ $k }}: {{ range $kk, $vv := $v }}{{ $kk }}: {{ $vv }}|{{ end -}}
{{ end }}


`)
					b.WithSourceFile("go.mod", `
module github.com/gohugoio/tests/testHugoModules


`)

					b.Build(BuildCfg{})

					// Verify that go.mod is autopopulated with all the modules in config.toml.
					b.AssertFileContent("go.mod", m.Path())

					b.AssertFileContent("public/index.html",
						"Mod Name: "+m.Name(),
						"Mod Version: v1.4.0")

					b.AssertFileContent("public/index.html", createChildModMatchers(m, ignoreVendor, m.Vendor)...)

				})

			}
		}
	}

}

func createChildModMatchers(m *mods.Md, ignoreVendor, vendored bool) []string {
	// Child depdendencies are one behind.
	expectMinorVersion := 3

	if !ignoreVendor && vendored {
		// Vendored modules are stuck at v1.1.0.
		expectMinorVersion = 1
	}

	expectVersion := fmt.Sprintf("v1.%d.0", expectMinorVersion)

	var matchers []string

	for _, mm := range m.Children {
		matchers = append(
			matchers,
			fmt.Sprintf("%s: name: %s|version: %s", mm.Name(), mm.Name(), expectVersion))
		matchers = append(matchers, createChildModMatchers(mm, ignoreVendor, vendored || mm.Vendor)...)
	}
	return matchers
}

func TestThemeWithContent(t *testing.T) {
	t.Parallel()

	b := newTestSitesBuilder(t).WithConfigFile("toml", `
baseURL="https://example.org"

defaultContentLanguage = "en"

[module]
[[module.imports]]
path="a"
[[module.imports.mounts]]
source="myacontent"
target="content/blog"
lang="en"
[[module.imports]]
path="b"
[[module.imports.mounts]]
source="mybcontent"
target="content/blog"
lang="nn"

[languages]

[languages.en]
title = "Title in English"
languageName = "English"
weight = 1
[languages.nn]
languageName = "Nynorsk"
weight = 2
title = "Tittel på nynorsk"
[languages.nb]
languageName = "Bokmål"
weight = 3
title = "Tittel på bokmål"
[languages.fr]
languageName = "French"
weight = 4
title = "French Title"


`)

	b.WithTemplatesAdded("index.html", `
{{ range .Site.RegularPages }}
|{{ .Title }}|{{ .RelPermalink }}|{{ .Plain }}
{{ end }}
`)

	b.WithSourceFile("themes/a/myacontent/page.md", `---
title: Theme Content A
---
Content A

`)

	b.WithSourceFile("themes/b/mybcontent/page.md", `---
title: Theme Content B
---
Content B

`)

	b.Build(BuildCfg{})

	b.AssertFileContent("public/index.html", "|Theme Content A|/blog/page/|Content A")
	b.AssertFileContent("public/nn/index.html", "|Theme Content B|/nn/blog/page/|Content B")

}
