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
	"sort"

	"github.com/gohugoio/hugo/config"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

var (
	componentFolders = []string{
		"archetypes",
		"static",
		"layouts",
		"content",
		"data",
		"i18n",
		"assets",
		"resources",
	}

	componentFoldersSet = make(map[string]bool)
)

func init() {
	sort.Strings(componentFolders)
	for _, f := range componentFolders {
		componentFoldersSet[f] = true
	}
}

type Config struct {
	Imports []Import
}

type Import struct {
	Path   string // Module path
	Mounts []Mount
}

type Mount struct {
	Source string // relative path in source repo, e.g. "scss"
	Target string // relative target path, e.g. "assets/bootstrap/scss"

	// TODO(bep) mod
	Lang string
}

// DecodeConfig creates a modules Config from a given Hugo configuration.
func DecodeConfig(cfg config.Provider) (c Config, err error) {
	if cfg == nil {
		return
	}

	themeSet := cfg.IsSet("theme")
	moduleSet := cfg.IsSet("module")

	if themeSet && moduleSet {
		return c, errors.New("ambigous module config; both 'theme' and 'module' provided")
	}

	if themeSet {
		imports := config.GetStringSlicePreserveString(cfg, "theme")
		for _, imp := range imports {
			c.Imports = append(c.Imports, Import{
				Path: imp,
			})
		}

		return
	}

	m := cfg.GetStringMap("module")

	err = mapstructure.WeakDecode(m, &c)

	return
}
