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
	"bytes"
	"testing"

	"github.com/gohugoio/hugo/htesting"

	"github.com/gohugoio/hugo/hugofs"

	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Parallel()

	modName := "hugo-modules-basic-test"
	modPath := "github.com/gohugoio/tests/" + modName
	modConfig := Config{
		Imports: []Import{Import{Path: "github.com/gohugoio/hugoTestModules1_darwin/modh2_2"}},
	}

	assert := require.New(t)

	workingDir, clean, err := htesting.CreateTempDir(hugofs.Os, modName)
	assert.NoError(err)
	defer clean()

	client := NewClient(ClientConfig{
		Fs:           hugofs.Os,
		WorkingDir:   workingDir,
		ModuleConfig: modConfig,
	})

	// Test Init
	assert.NoError(client.Init(modPath))

	// Test Collect
	mc, err := client.Collect()
	assert.NoError(err)
	assert.Equal(3, len(mc.Modules))

	// Test Graph
	var graphb bytes.Buffer
	assert.NoError(client.Graph(&graphb))

	expect := `github.com/gohugoio/tests/hugo-modules-basic-test github.com/gohugoio/hugoTestModules1_darwin/modh2_2@v1.4.0
github.com/gohugoio/hugoTestModules1_darwin/modh2_2@v1.4.0 github.com/gohugoio/hugoTestModules1_darwin/modh2_2_1v@v1.3.0
github.com/gohugoio/hugoTestModules1_darwin/modh2_2@v1.4.0 github.com/gohugoio/hugoTestModules1_darwin/modh2_2_2@v1.3.0
`

	assert.Equal(expect, graphb.String())

	// Test Vendor
	assert.NoError(client.Vendor())
	graphb.Reset()
	assert.NoError(client.Graph(&graphb))
	expectVendored := `github.com/gohugoio/tests/hugo-modules-basic-test github.com/gohugoio/hugoTestModules1_darwin/modh2_2@v1.4.0+vendor
github.com/gohugoio/tests/hugo-modules-basic-test github.com/gohugoio/hugoTestModules1_darwin/modh2_2_1v@v1.3.0+vendor
github.com/gohugoio/tests/hugo-modules-basic-test github.com/gohugoio/hugoTestModules1_darwin/modh2_2_2@v1.3.0+vendor
`
	assert.Equal(expectVendored, graphb.String())

	// Test the ignoreVendor setting
	clientIgnoreVendor := NewClient(ClientConfig{
		Fs:           hugofs.Os,
		WorkingDir:   workingDir,
		ModuleConfig: modConfig,
		IgnoreVendor: true,
	})

	graphb.Reset()
	assert.NoError(clientIgnoreVendor.Graph(&graphb))
	assert.Equal(expect, graphb.String())

	// Test Tidy
	// TODO(bep) improve
	assert.NoError(client.Tidy())

}

func TestSetEnvVars(t *testing.T) {
	t.Parallel()
	assert := require.New(t)
	vars := []string{"FOO=bar", "HUGO=cool", "BAR=foo"}
	setEnvVars(&vars, "HUGO", "rocking!", "NEW", "bar")
	assert.Equal([]string{"FOO=bar", "HUGO=rocking!", "BAR=foo", "NEW=bar"}, vars)
}
