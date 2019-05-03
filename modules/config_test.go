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

package modules

import (
	"testing"

	"github.com/gohugoio/hugo/config"

	"github.com/stretchr/testify/require"
)

func TestDecodeConfig(t *testing.T) {
	assert := require.New(t)
	tomlConfig := `
[module]
[[module.imports]]
path="github.com/bep/mycomponent"
[[module.imports.mounts]]
source="scss"
target="assets/bootstrap/scss"
[[module.imports.mounts]]
source="src/markdown/blog"
target="content/blog"
lang="en"
`
	cfg, err := config.FromConfigString(tomlConfig, "toml")
	assert.NoError(err)

	mcfg, err := DecodeConfig(cfg)
	assert.NoError(err)

	assert.Len(mcfg.Imports, 1)
	imp := mcfg.Imports[0]
	imp.Path = "github.com/bep/mycomponent"
	assert.Equal("src/markdown/blog", imp.Mounts[1].Source)
	assert.Equal("content/blog", imp.Mounts[1].Target)
	assert.Equal("en", imp.Mounts[1].Lang)

}

// Test old style theme import.
func TestDecodeConfigTheme(t *testing.T) {
	assert := require.New(t)
	tomlConfig := `

theme = ["a", "b"]
`
	cfg, err := config.FromConfigString(tomlConfig, "toml")
	assert.NoError(err)

	mcfg, err := DecodeConfig(cfg)
	assert.NoError(err)

	assert.Len(mcfg.Imports, 2)
	assert.Equal("a", mcfg.Imports[0].Path)
	assert.Equal("b", mcfg.Imports[1].Path)
}

func TestDecodeConfigBothOldAndNewProvided(t *testing.T) {
	assert := require.New(t)
	tomlConfig := `

theme = ["a", "b"]

[module]
[[module.imports]]
path="github.com/bep/mycomponent"

`
	cfg, err := config.FromConfigString(tomlConfig, "toml")
	assert.NoError(err)

	_, err = DecodeConfig(cfg)
	assert.Error(err)

}

func TestComponentFolders(t *testing.T) {
	assert := require.New(t)

	// It's important that these are absolutely right and not changed.
	assert.Equal(len(componentFolders), len(componentFoldersSet))
	assert.True(componentFoldersSet["archetypes"])
	assert.True(componentFoldersSet["layouts"])
	assert.True(componentFoldersSet["data"])
	assert.True(componentFoldersSet["i18n"])
	assert.True(componentFoldersSet["assets"])
	assert.True(componentFoldersSet["resources"])
	assert.True(componentFoldersSet["static"])
	assert.True(componentFoldersSet["content"])

}
