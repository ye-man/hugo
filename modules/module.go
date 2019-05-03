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

// Package modules provides a client that can be used to manage Hugo Components,
// what's refered to as Hugo Modules. Hugo Modules is built on top of Go Modules,
// but also supports vendoring and components stored directly in the themes dir.
package modules

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gohugoio/hugo/config"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

var _ Module = (*moduleAdapter)(nil)

type Module interface {

	// Optional config read from the configFilename above.
	Cfg() config.Provider

	// Optional configuration filename (e.g. "/themes/mytheme/config.json").
	// This will be added to the special configuration watch list when in
	// server mode.
	ConfigFilename() string

	// Directory holding files for this module.
	Dir() string

	// Returns whether this is a Go Module.
	IsGoMod() bool

	// In the dependency tree, this is the first module that defines this module
	// as a dependency.
	Owner() Module

	// Replaced by this module.
	Replace() Module

	// Any directory remappings.
	Mounts() []Mount

	// Returns the path to this module.
	// This will either be the module path, e.g. "github.com/gohugoio/myshortcodes",
	// or the path below your /theme folder, e.g. "mytheme".
	Path() string

	// Returns whether Dir points below the _vendor dir.
	Vendor() bool

	// The module version.
	Version() string
}

type Modules []Module

type moduleAdapter struct {
	path      string
	dir       string
	version   string
	vendor    bool
	owner     Module
	modImport Import

	configFilename string
	cfg            config.Provider

	// Set if a Go module.
	gomod *goModule
}

func (m *moduleAdapter) Cfg() config.Provider {
	return m.cfg
}

func (m *moduleAdapter) ConfigFilename() string {
	return m.configFilename
}

func (m *moduleAdapter) Dir() string {
	// This may point to the _vendor dir.
	if !m.IsGoMod() || m.dir != "" {
		return m.dir
	}
	return m.gomod.Dir
}

func (m *moduleAdapter) IsGoMod() bool {
	return m.gomod != nil
}

func (m *moduleAdapter) Owner() Module {
	return m.owner
}

func (m *moduleAdapter) Replace() Module {
	if m.IsGoMod() && !m.Vendor() && m.gomod.Replace != nil {
		return &moduleAdapter{
			gomod: m.gomod.Replace,
			owner: m.owner,
		}
	}
	return nil
}

func (m *moduleAdapter) Mounts() []Mount {
	return m.modImport.Mounts
}

func (m *moduleAdapter) Path() string {
	if !m.IsGoMod() || m.path != "" {
		return m.path
	}
	return m.gomod.Path
}

func (m *moduleAdapter) Vendor() bool {
	return m.vendor
}

func (m *moduleAdapter) Version() string {
	if !m.IsGoMod() || m.version != "" {
		return m.version
	}
	return m.gomod.Version
}

func (m *moduleAdapter) validateAndApplyDefaults(fs afero.Fs) error {

	baseErr := errors.Errorf("invalid module config for %q", m.Path())
	dir := m.Dir()

	for _, mnt := range m.modImport.Mounts {
		if mnt.Source == "" || mnt.Target == "" {
			return errors.Wrap(baseErr, "both source and target must be set")
		}

		mnt.Source = filepath.Clean(mnt.Source)
		mnt.Target = filepath.Clean(mnt.Target)

		// Verify that Source exists
		sourceDir := filepath.Join(dir, mnt.Source)
		_, err := fs.Stat(sourceDir)
		if err != nil {
			return errors.Wrapf(baseErr, "module mount source not found: %q", mnt.Source)
		}

		// Verify that target points to one of the predefined component dirs
		targetBase := mnt.Target
		idxPathSep := strings.Index(mnt.Target, string(os.PathSeparator))
		if idxPathSep != -1 {
			targetBase = mnt.Target[0:idxPathSep]
		}
		if !componentFoldersSet[targetBase] {
			return errors.Wrapf(baseErr, "mount target must be one of: %v", componentFolders)
		}
	}

	if len(m.modImport.Mounts) == 0 {
		// Create default mount points for every component folder that
		// exists in the module.
		for _, cf := range componentFolders {
			sourceDir := filepath.Join(dir, cf)
			_, err := fs.Stat(sourceDir)
			if err == nil {
				m.modImport.Mounts = append(m.modImport.Mounts, Mount{
					Source: cf,
					Target: cf,
				})
			}
		}
	}

	return nil

}
