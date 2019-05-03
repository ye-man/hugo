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

package commands

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gohugoio/hugo/config"

	"github.com/gohugoio/hugo/hugofs"

	"github.com/gohugoio/hugo/modules"
	"github.com/spf13/cobra"
)

var _ cmder = (*modCmd)(nil)

type modCmd struct {
	*baseBuilderCmd
}

func (b *commandsBuilder) newModCmd() *modCmd {
	c := &modCmd{}

	const commonUsage = `
Note that Hugo will always start out by resolving the components defined in the site
configuration, provided by a _vendor directory (if no --ignoreVendor flag provided),
Go Modules, or a folder inside the themes directory, in that order.

`

	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Various Hugo Modules helpers.",
		Long:  "LONG V",

		RunE: nil,
	}

	cmd.AddCommand(
		&cobra.Command{
			// go get [-d] [-m] [-u] [-v] [-insecure] [build flags] [packages]
			Use:                "get",
			DisableFlagParsing: true,
			Short:              "Resolves dependencies in your current Hugo Project.",
			Long: `
Resolves dependencies in your current Hugo Project.

You 'go get github.com/gohugoio/testshortcodes@v0.3.0'

Run "go help get" for more information.
` + commonUsage,
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.withModsClient(func(c *modules.Client) error {
					// We currently just pass on the flags we get to Go and
					// need to do the flag handling manually.
					if len(args) == 1 && strings.Contains(args[0], "-h") {
						return cmd.Help()
					}
					return c.Get(args...)
				})
			},
		},
		&cobra.Command{
			Use:   "graph",
			Short: "TODO(bep)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.withModsClient(func(c *modules.Client) error {
					return c.Graph(os.Stdout)
				})
			},
		},
		&cobra.Command{
			Use:   "init",
			Short: "TODO(bep) ",
			RunE: func(cmd *cobra.Command, args []string) error {
				var path string
				if len(args) >= 1 {
					path = args[0]
				}
				return c.withModsClient(func(c *modules.Client) error {
					return c.Init(path)
				})
			},
		},
		&cobra.Command{
			Use:   "vendor",
			Short: "TODO(bep)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.withModsClient(func(c *modules.Client) error {
					return c.Vendor()
				})
			},
		},
		&cobra.Command{
			Use:   "tidy",
			Short: "TODO(bep)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.withModsClient(func(c *modules.Client) error {
					return c.Tidy()
				})
			},
		},
	)

	c.baseBuilderCmd = b.newBuilderCmd(cmd)

	return c

}

func (c *modCmd) withModsClient(f func(*modules.Client) error) error {
	com, err := c.initConfig()
	if err != nil {
		return err
	}
	client, err := c.newModsClient(com.Cfg)
	if err != nil {
		return err
	}
	return f(client)
}

func (c *modCmd) initConfig() (*commandeer, error) {
	com, err := initializeConfig(true, false, &c.hugoBuilderCommon, c, nil)
	if err != nil {
		return nil, err
	}
	return com, nil
}

func (c *modCmd) newModsClient(cfg config.Provider) (*modules.Client, error) {
	var (
		workingDir   string
		themesDir    string
		modProxy     string
		modConfig    modules.Config
		ignoreVendor bool
	)

	if c.source != "" {
		workingDir = c.source
	} else {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	if cfg != nil {
		themesDir = cfg.GetString("themesDir")
		if themesDir != "" && !filepath.IsAbs(themesDir) {
			themesDir = filepath.Join(workingDir, themesDir)
		}
		var err error
		modConfig, err = modules.DecodeConfig(cfg)
		if err != nil {
			return nil, err
		}
		ignoreVendor = cfg.GetBool("ignoreVendor")
		modProxy = cfg.GetString("modProxy")
	}

	return modules.NewClient(modules.ClientConfig{
		Fs:           hugofs.Os,
		WorkingDir:   workingDir,
		ThemesDir:    themesDir,
		ModuleConfig: modConfig,
		IgnoreVendor: ignoreVendor,
		ModProxy:     modProxy,
	}), nil

}
