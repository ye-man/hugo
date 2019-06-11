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

// Package filesystems provides the fine grained file systems used by Hugo. These
// are typically virtual filesystems that are composites of project and theme content.
package filesystems

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/gohugoio/hugo/modules"

	"github.com/gohugoio/hugo/config"

	"github.com/gohugoio/hugo/hugofs"

	"fmt"

	"github.com/gohugoio/hugo/hugolib/paths"
	"github.com/gohugoio/hugo/langs"
	"github.com/spf13/afero"
)

// When we create a virtual filesystem with data and i18n bundles for the project and the themes,
// this is the name of the project's virtual root. It got it's funky name to make sure
// (or very unlikely) that it collides with a theme name.
const projectVirtualFolder = "__h__project"

var filePathSeparator = string(filepath.Separator)

// BaseFs contains the core base filesystems used by Hugo. The name "base" is used
// to underline that even if they can be composites, they all have a base path set to a specific
// resource folder, e.g "/my-project/content". So, no absolute filenames needed.
type BaseFs struct {

	// SourceFilesystems contains the different source file systems.
	*SourceFilesystems

	// The filesystem used to publish the rendered site.
	// This usually maps to /my-project/public.
	PublishFs afero.Fs

	themeFs themeFilesystems

	// TODO(bep) improve the "theme interaction"
	AbsThemeDirs []string
}

// RelContentDir tries to create a path relative to the content root from
// the given filename. The return value is the path and language code.
func (b *BaseFs) RelContentDir(filename string) string {
	for _, dirname := range b.SourceFilesystems.Content.Dirnames {
		if strings.HasPrefix(filename, dirname) {
			rel := strings.TrimPrefix(filename, dirname)
			return strings.TrimPrefix(rel, filePathSeparator)
		}
	}
	// Either not a content dir or already relative.
	return filename
}

// SourceFilesystems contains the different source file systems. These can be
// composite file systems (theme and project etc.), and they have all root
// set to the source type the provides: data, i18n, static, layouts.
type SourceFilesystems struct {
	Content    *SourceFilesystem
	Data       *SourceFilesystem
	I18n       *SourceFilesystem
	Layouts    *SourceFilesystem
	Archetypes *SourceFilesystem
	Assets     *SourceFilesystem
	Resources  *SourceFilesystem

	// This is a unified read-only view of the project's and themes' workdir.
	Work *SourceFilesystem

	// When in multihost we have one static filesystem per language. The sync
	// static files is currently done outside of the Hugo build (where there is
	// a concept of a site per language).
	// When in non-multihost mode there will be one entry in this map with a blank key.
	Static map[string]*SourceFilesystem
}

// A SourceFilesystem holds the filesystem for a given source type in Hugo (data,
// i18n, layouts, static) and additional metadata to be able to use that filesystem
// in server mode.
type SourceFilesystem struct {
	// This is a virtual composite filesystem. It expects path relative to a context.
	Fs afero.Fs

	// This is the base source filesystem. In real Hugo, this will be the OS filesystem.
	// Use this if you need to resolve items in Dirnames below.
	SourceFs afero.Fs

	// Dirnames is absolute filenames to the directories in this filesystem.
	Dirnames []string

	// When syncing a source folder to the target (e.g. /public), this may
	// be set to publish into a subfolder. This is used for static syncing
	// in multihost mode.
	PublishFolder string
}

// ContentStaticAssetFs will create a new composite filesystem from the content,
// static, and asset filesystems. The site language is needed to pick the correct static filesystem.
// The order is content, static and then assets.
// TODO(bep) check usage
func (s SourceFilesystems) ContentStaticAssetFs(lang string) afero.Fs {
	staticFs := s.StaticFs(lang)

	base := afero.NewCopyOnWriteFs(s.Assets.Fs, staticFs)
	return afero.NewCopyOnWriteFs(base, s.Content.Fs)

}

// StaticFs returns the static filesystem for the given language.
// This can be a composite filesystem.
func (s SourceFilesystems) StaticFs(lang string) afero.Fs {
	var staticFs afero.Fs = hugofs.NoOpFs

	if fs, ok := s.Static[lang]; ok {
		staticFs = fs.Fs
	} else if fs, ok := s.Static[""]; ok {
		staticFs = fs.Fs
	}

	return staticFs
}

// StatResource looks for a resource in these filesystems in order: static, assets and finally content.
// If found in any of them, it returns FileInfo and the relevant filesystem.
// Any non os.IsNotExist error will be returned.
// An os.IsNotExist error wil be returned only if all filesystems return such an error.
// Note that if we only wanted to find the file, we could create a composite Afero fs,
// but we also need to know which filesystem root it lives in.
func (s SourceFilesystems) StatResource(lang, filename string) (fi os.FileInfo, fs afero.Fs, err error) {
	for _, fsToCheck := range []afero.Fs{s.StaticFs(lang), s.Assets.Fs, s.Content.Fs} {
		fs = fsToCheck
		fi, err = fs.Stat(filename)
		if err == nil || !os.IsNotExist(err) {
			return
		}
	}
	// Not found.
	return
}

// IsStatic returns true if the given filename is a member of one of the static
// filesystems.
func (s SourceFilesystems) IsStatic(filename string) bool {
	for _, staticFs := range s.Static {
		if staticFs.Contains(filename) {
			return true
		}
	}
	return false
}

// IsContent returns true if the given filename is a member of the content filesystem.
func (s SourceFilesystems) IsContent(filename string) bool {
	return s.Content.Contains(filename)
}

// IsLayout returns true if the given filename is a member of the layouts filesystem.
func (s SourceFilesystems) IsLayout(filename string) bool {
	return s.Layouts.Contains(filename)
}

// IsData returns true if the given filename is a member of the data filesystem.
func (s SourceFilesystems) IsData(filename string) bool {
	return s.Data.Contains(filename)
}

// IsAsset returns true if the given filename is a member of the asset filesystem.
func (s SourceFilesystems) IsAsset(filename string) bool {
	return s.Assets.Contains(filename)
}

// IsI18n returns true if the given filename is a member of the i18n filesystem.
func (s SourceFilesystems) IsI18n(filename string) bool {
	return s.I18n.Contains(filename)
}

// MakeStaticPathRelative makes an absolute static filename into a relative one.
// It will return an empty string if the filename is not a member of a static filesystem.
func (s SourceFilesystems) MakeStaticPathRelative(filename string) string {
	for _, staticFs := range s.Static {
		rel := staticFs.MakePathRelative(filename)
		if rel != "" {
			return rel
		}
	}
	return ""
}

// MakePathRelative creates a relative path from the given filename.
// It will return an empty string if the filename is not a member of this filesystem.
func (d *SourceFilesystem) MakePathRelative(filename string) string {
	for _, currentPath := range d.Dirnames {
		if strings.HasPrefix(filename, currentPath) {
			return strings.TrimPrefix(filename, currentPath)
		}
	}
	return ""
}

func (d *SourceFilesystem) RealFilename(rel string) string {
	fi, err := d.Fs.Stat(rel)
	if err != nil {
		return rel
	}
	if realfi, ok := fi.(hugofs.RealFilenameInfo); ok {
		return realfi.RealFilename()
	}

	return rel
}

// Contains returns whether the given filename is a member of the current filesystem.
func (d *SourceFilesystem) Contains(filename string) bool {
	for _, dir := range d.Dirnames {
		if strings.HasPrefix(filename, dir) {
			return true
		}
	}
	return false
}

// RealDirs gets a list of absolute paths to directories starting from the given
// path.
func (d *SourceFilesystem) RealDirs(from string) []string {
	var dirnames []string
	for _, dir := range d.Dirnames {
		dirname := filepath.Join(dir, from)
		if _, err := d.SourceFs.Stat(dirname); err == nil {
			dirnames = append(dirnames, dirname)
		}
	}
	return dirnames
}

// WithBaseFs allows reuse of some potentially expensive to create parts that remain
// the same across sites/languages.
func WithBaseFs(b *BaseFs) func(*BaseFs) error {
	return func(bb *BaseFs) error {
		bb.themeFs = b.themeFs
		bb.AbsThemeDirs = b.AbsThemeDirs
		return nil
	}
}

func newRealBase(base afero.Fs) afero.Fs {
	return hugofs.NewBasePathRealFilenameFs(base.(*afero.BasePathFs))

}

// NewBase builds the filesystems used by Hugo given the paths and options provided.NewBase
func NewBase(p *paths.Paths, options ...func(*BaseFs) error) (*BaseFs, error) {
	fs := p.Fs

	publishFs := afero.NewBasePathFs(fs.Destination, p.AbsPublishDir)

	b := &BaseFs{
		PublishFs: publishFs,
	}

	for _, opt := range options {
		if err := opt(b); err != nil {
			return nil, err
		}
	}

	builder := newSourceFilesystemsBuilder(p, b)
	sourceFilesystems, err := builder.Build()
	if err != nil {
		return nil, err
	}

	b.SourceFilesystems = sourceFilesystems
	b.themeFs = builder.themeFs
	// TODO(bep) mod	b.AbsThemeDirs = builder.absThemeDirs

	return b, nil
}

type sourceFilesystemsBuilder struct {
	p       *paths.Paths
	result  *SourceFilesystems
	themeFs themeFilesystems
}

func newSourceFilesystemsBuilder(p *paths.Paths, b *BaseFs) *sourceFilesystemsBuilder {
	return &sourceFilesystemsBuilder{p: p, themeFs: b.themeFs, result: &SourceFilesystems{}}
}

func (b *sourceFilesystemsBuilder) Build() (*SourceFilesystems, error) {
	if b.themeFs.overlay == nil && b.hasModules() {
		themeFs, err := b.createThemesOverlayFs2(b.p)
		if err != nil {
			return nil, err
		}
		if themeFs.overlay == nil {
			panic("createThemesFs returned nil")
		}
		b.themeFs = themeFs
	}

	cfs, err := b.createContentFsM()
	if err != nil {
		return nil, err
	}
	b.result.Content = cfs

	sfs, err := b.createRootMappingFs("dataDir", "data")
	if err != nil {
		return nil, err
	}
	b.result.Data = sfs

	sfs, err = b.createRootMappingFs("i18nDir", "i18n")
	if err != nil {
		return nil, err
	}
	b.result.I18n = sfs

	sfs, err = b.createFs(false, true, "layoutDir", "layouts")
	if err != nil {
		return nil, err
	}
	b.result.Layouts = sfs

	sfs, err = b.createFs(false, true, "archetypeDir", "archetypes")
	if err != nil {
		return nil, err
	}
	b.result.Archetypes = sfs

	sfs, err = b.createFs(false, true, "assetDir", "assets")
	if err != nil {
		return nil, err
	}
	b.result.Assets = sfs

	sfs, err = b.createFs(true, false, "resourceDir", "resources")
	if err != nil {
		return nil, err
	}

	b.result.Resources = sfs

	sfs, err = b.createFs(false, true, "", "")
	if err != nil {
		return nil, err
	}
	b.result.Work = sfs

	err = b.createStaticFs()
	if err != nil {
		return nil, err
	}

	return b.result, nil
}

func (b *sourceFilesystemsBuilder) createContentFsM() (*SourceFilesystem, error) {
	s := &SourceFilesystem{
		SourceFs: b.p.Fs.Source,
	}

	projectContent, absContentDirs, err := b.createContentFs(
		s.SourceFs,
		b.p.WorkingDir,
		b.p.DefaultContentLanguage,
		b.p.Languages)

	if err != nil {
		return nil, err
	}

	// Make sure we don't have any overlapping content dirs. That will never work.
	for i, d1 := range absContentDirs {
		for j, d2 := range absContentDirs {
			if i == j {
				continue
			}
			if strings.HasPrefix(d1, d2) || strings.HasPrefix(d2, d1) {
				return nil, fmt.Errorf("found overlapping content dirs (%q and %q)", d1, d2)
			}
		}
	}

	// TODO(bep) mod contentFilesystems probably needs to be reversed
	contentFs, err := hugofs.NewLanguageFs(b.p.Languages.AsSet(), append(projectContent, b.themeFs.contentFilesystems...)...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create content filesystem")
	}

	// TODO(bep) mod check created once

	//printFs(contentFs, "", os.Stdout)

	s.Fs = contentFs
	s.Dirnames = absContentDirs

	return s, nil
}

func (b *sourceFilesystemsBuilder) createFs(
	mkdir bool,
	readOnly bool,
	dirKey, themeFolder string) (*SourceFilesystem, error) {
	s := &SourceFilesystem{
		SourceFs: b.p.Fs.Source,
	}

	if themeFolder == "" {
		themeFolder = filePathSeparator
	}

	var dir string
	if dirKey != "" {
		dir = b.p.Cfg.GetString(dirKey)
		if dir == "" {
			return s, fmt.Errorf("config %q not set", dirKey)
		}
	}

	var fs afero.Fs

	absDir := b.p.AbsPathify(dir)
	existsInSource := b.existsInSource(absDir)
	if !existsInSource && mkdir {
		// We really need this directory. Make it.
		if err := b.p.Fs.Source.MkdirAll(absDir, 0777); err == nil {
			existsInSource = true
		}
	}
	if existsInSource {
		fs = newRealBase(afero.NewBasePathFs(b.p.Fs.Source, absDir))
		s.Dirnames = []string{absDir}
	}

	if b.hasModules() {
		if !strings.HasPrefix(themeFolder, filePathSeparator) {
			themeFolder = filePathSeparator + themeFolder
		}
		bfs := afero.NewBasePathFs(b.themeFs.overlay, themeFolder)
		themeFolderFs := newRealBase(bfs)

		if fs == nil {
			fs = themeFolderFs
		} else {
			fs = afero.NewCopyOnWriteFs(themeFolderFs, fs)
		}

		// TODO(bep) mod
		/*
			for _, absThemeDir := range b.absThemeDirs {
				absThemeFolderDir := filepath.Join(absThemeDir, themeFolder)
				if b.existsInSource(absThemeFolderDir) {
					s.Dirnames = append(s.Dirnames, absThemeFolderDir)
				}
			}
		*/
	}

	if fs == nil {
		s.Fs = hugofs.NoOpFs
	} else if readOnly {
		s.Fs = afero.NewReadOnlyFs(fs)
	} else {
		s.Fs = fs
	}

	return s, nil
}

// Used for data, i18n -- we cannot use overlay filsesystems for those, but we need
// to keep a strict order.
// TODO(bep) mod remove
func (b *sourceFilesystemsBuilder) createRootMappingFs(dirKey, themeFolder string) (*SourceFilesystem, error) {
	s := &SourceFilesystem{
		SourceFs: b.p.Fs.Source,
	}

	projectDir := b.p.Cfg.GetString(dirKey)
	if projectDir == "" {
		return nil, fmt.Errorf("config %q not set", dirKey)
	}

	var fromTo []string
	to := b.p.AbsPathify(projectDir)

	if b.existsInSource(to) {
		s.Dirnames = []string{to}
		fromTo = []string{projectVirtualFolder, to}
	}

	for _, module := range b.p.AllModules {
		mounts := module.Mounts()
		if len(mounts) == 0 {
			// Create one for this sub folder
			mounts = []modules.Mount{modules.Mount{
				Target: themeFolder,
				Source: themeFolder,
			}}
		}

		for _, mount := range mounts {
			if !strings.HasPrefix(mount.Target, themeFolder) {
				// Not part of this filesystem
				continue
			}
			to := filepath.Join(module.Dir(), mount.Source)
			if b.existsInSource(to) {
				// TODO(bep) mod Dirnames is used for watching, maybe
				// not applicable to all.

				s.Dirnames = append(s.Dirnames, to)
				from := filepath.Join(module.Path(), strings.TrimPrefix(mount.Target, themeFolder))
				fromTo = append(fromTo, from, to)
			}

		}
	}

	if len(fromTo) == 0 {
		s.Fs = hugofs.NoOpFs
		return s, nil
	}

	fs, err := hugofs.NewRootMappingFsFromFromTo(b.p.Fs.Source, fromTo...)
	if err != nil {
		return nil, err
	}

	s.Fs = afero.NewReadOnlyFs(fs)

	return s, nil
}

func (b *sourceFilesystemsBuilder) hasModules() bool {
	return len(b.p.AllModules) > 0
}

func (b *sourceFilesystemsBuilder) existsInSource(abspath string) bool {
	exists, _ := afero.Exists(b.p.Fs.Source, abspath)
	return exists
}

func (b *sourceFilesystemsBuilder) createStaticFs() error {
	isMultihost := b.p.Cfg.GetBool("multihost")
	ms := make(map[string]*SourceFilesystem)
	b.result.Static = ms

	if isMultihost {
		for _, l := range b.p.Languages {
			s := &SourceFilesystem{
				SourceFs:      b.p.Fs.Source,
				PublishFolder: l.Lang}
			staticDirs := removeDuplicatesKeepRight(getStaticDirs(l))
			if len(staticDirs) == 0 {
				continue
			}

			for _, dir := range staticDirs {
				absDir := b.p.AbsPathify(dir)
				if !b.existsInSource(absDir) {
					continue
				}

				s.Dirnames = append(s.Dirnames, absDir)
			}

			fs, err := createOverlayFs(b.p.Fs.Source, s.Dirnames)
			if err != nil {
				return err
			}

			if b.hasModules() {
				themeFolder := "static"
				fs = afero.NewCopyOnWriteFs(newRealBase(afero.NewBasePathFs(b.themeFs.overlay, themeFolder)), fs)
				// TODO(bep) mod
				/*for _, absThemeDir := range b.absThemeDirs {
					s.Dirnames = append(s.Dirnames, filepath.Join(absThemeDir, themeFolder))
				}*/
			}

			s.Fs = fs
			ms[l.Lang] = s

		}

		return nil
	}

	s := &SourceFilesystem{
		SourceFs: b.p.Fs.Source,
	}

	var staticDirs []string

	for _, l := range b.p.Languages {
		staticDirs = append(staticDirs, getStaticDirs(l)...)
	}

	staticDirs = removeDuplicatesKeepRight(staticDirs)
	if len(staticDirs) == 0 {
		return nil
	}

	for _, dir := range staticDirs {
		absDir := b.p.AbsPathify(dir)
		if !b.existsInSource(absDir) {
			continue
		}
		s.Dirnames = append(s.Dirnames, absDir)
	}

	fs, err := createOverlayFs(b.p.Fs.Source, s.Dirnames)
	if err != nil {
		return err
	}

	if b.hasModules() {
		themeFolder := "static"
		fs = afero.NewCopyOnWriteFs(newRealBase(afero.NewBasePathFs(b.themeFs.overlay, themeFolder)), fs)
		// TODO(bep) mod
		/*for _, absThemeDir := range b.absThemeDirs {
			s.Dirnames = append(s.Dirnames, filepath.Join(absThemeDir, themeFolder))
		}*/
	}

	s.Fs = fs
	ms[""] = s

	return nil
}

func getStaticDirs(cfg config.Provider) []string {
	var staticDirs []string
	for i := -1; i <= 10; i++ {
		staticDirs = append(staticDirs, getStringOrStringSlice(cfg, "staticDir", i)...)
	}
	return staticDirs
}

func getStringOrStringSlice(cfg config.Provider, key string, id int) []string {

	if id >= 0 {
		key = fmt.Sprintf("%s%d", key, id)
	}

	return config.GetStringSlicePreserveString(cfg, key)

}

func (b *sourceFilesystemsBuilder) createContentFs(fs afero.Fs,
	workingDir,
	defaultContentLanguage string,
	languages langs.Languages) ([]hugofs.LangFsProvider, []string, error) {

	var contentLanguages langs.Languages
	var contentDirSeen = make(map[string]bool)
	languageSet := make(map[string]bool)

	// The default content language needs to be first.
	for _, language := range languages {
		if language.Lang == defaultContentLanguage {
			contentLanguages = append(contentLanguages, language)
			contentDirSeen[language.ContentDir] = true
		}
		languageSet[language.Lang] = true
	}

	for _, language := range languages {
		if contentDirSeen[language.ContentDir] {
			continue
		}
		if language.ContentDir == "" {
			language.ContentDir = defaultContentLanguage
		}
		contentDirSeen[language.ContentDir] = true
		contentLanguages = append(contentLanguages, language)

	}

	var absContentDirs []string

	langFsProviders, err := b.createContentFilesystems(fs, workingDir, contentLanguages, languageSet, &absContentDirs)
	return langFsProviders, absContentDirs, err

}

func (b *sourceFilesystemsBuilder) createContentFilesystems(source afero.Fs,
	workingDir string,
	languages langs.Languages,
	languageSet map[string]bool,
	absContentDirs *[]string) ([]hugofs.LangFsProvider, error) {

	var langFsProviders []hugofs.LangFsProvider

	for _, language := range languages {

		contentDir := language.ContentDir
		if contentDir == "" {
			panic("missing contentDir")
		}

		absContentDir := paths.AbsPathify(workingDir, language.ContentDir)
		if !strings.HasSuffix(absContentDir, paths.FilePathSeparator) {
			absContentDir += paths.FilePathSeparator
		}

		// If root, remove the second '/'
		if absContentDir == "//" {
			absContentDir = paths.FilePathSeparator
		}

		if len(absContentDir) < 6 {
			return nil, fmt.Errorf("invalid content dir %q: Path is too short", absContentDir)
		}

		*absContentDirs = append(*absContentDirs, absContentDir)

		bfs := newRealBase(afero.NewBasePathFs(source, absContentDir))

		langFsProviders = append(langFsProviders, hugofs.NewLangFsProvider(language.Lang, bfs))

	}

	return langFsProviders, nil

}

type themeFilesystems struct {
	overlay afero.Fs // Complete view

	contentFilesystems []hugofs.LangFsProvider // Partitioned by language

	watchDirs []string // TODO(bep) mod
}

func (b *sourceFilesystemsBuilder) createThemesOverlayFs2(p *paths.Paths) (themeFilesystems, error) {

	mods := p.AllModules

	if len(mods) == 0 {
		panic("AllModules not set")
	}

	modsReversed := make(modules.Modules, len(mods))

	// The themes are ordered from left to right. We need to revert it to get the
	// overlay logic below working as expected.
	for i := 0; i < len(mods); i++ {
		modsReversed[i] = mods[len(mods)-1-i]
	}

	fs, contentFss, err := b.createThemeOverlayFs(p.Fs.Source, modsReversed)
	//fs = hugofs.NewNoLstatFs(fs) // TODO(bep) mod
	return themeFilesystems{
		overlay:            fs,
		contentFilesystems: contentFss,
	}, err

}

func (b *sourceFilesystemsBuilder) isContentMount(mnt modules.Mount) bool {
	return strings.HasPrefix(mnt.Target, "content")
}

func (b *sourceFilesystemsBuilder) createModFs(source afero.Fs, mod modules.Module) (*hugofs.RootMappingFs, bool, error) {

	var fromTo []hugofs.RootMapping
	var hasContentMount bool

	for _, mount := range mod.Mounts() {
		fmt.Println(">>> SOURCE/TARGET", mount.Source, mount.Target)
		rm := hugofs.RootMapping{
			From: mount.Target,
			To:   mount.Source,
		}

		if b.isContentMount(mount) {
			hasContentMount = true
			lang := mount.Lang
			if lang == "" {
				lang = b.p.DefaultContentLanguage
			}
			rm.Lang = lang
		}
		fromTo = append(fromTo, rm)
	}

	modBase := newRealBase(afero.NewBasePathFs(source, mod.Dir()))

	rmfs, err := hugofs.NewRootMappingFs(modBase, fromTo...)
	if err != nil {
		return nil, hasContentMount, err
	}

	return rmfs, hasContentMount, nil

}

func printFs(fs afero.Fs, path string, w io.Writer) {
	if fs == nil {
		return
	}
	afero.Walk(fs, path, func(path string, info os.FileInfo, err error) error {
		fmt.Println("p:::", path)
		return nil
	})
}

const contentBase = "content"

func (b *sourceFilesystemsBuilder) createThemeOverlayFs(source afero.Fs, mods modules.Modules) (afero.Fs, []hugofs.LangFsProvider, error) {
	if len(mods) == 0 {
		return hugofs.NoOpFs, nil, nil
	}

	var contentFss []hugofs.LangFsProvider

	base, hasContent, err := b.createModFs(source, mods[0])
	if err != nil {
		return nil, nil, err
	}
	if hasContent {
		ldirs, err := base.Dirs(contentBase)
		if err != nil {
			return nil, nil, err
		}
		contentFss = append(contentFss, hugofs.ToLangFsProviders(ldirs)...)
	}

	if len(mods) == 1 {
		return base, contentFss, nil
	}

	overlay, overlayContentFss, err := b.createThemeOverlayFs(source, mods[1:])
	if err != nil {
		return nil, nil, err
	}

	contentFss = append(contentFss, overlayContentFss...)

	return afero.NewCopyOnWriteFs(base, overlay), contentFss, nil
}

// TODO(bep) mod remove
func createOverlayFs(source afero.Fs, absPaths []string) (afero.Fs, error) {
	if len(absPaths) == 0 {
		return hugofs.NoOpFs, nil
	}

	if len(absPaths) == 1 {
		return afero.NewReadOnlyFs(newRealBase(afero.NewBasePathFs(source, absPaths[0]))), nil
	}

	base := afero.NewReadOnlyFs(newRealBase(afero.NewBasePathFs(source, absPaths[0])))
	overlay, err := createOverlayFs(source, absPaths[1:])
	if err != nil {
		return nil, err
	}

	return afero.NewCopyOnWriteFs(base, overlay), nil
}

func removeDuplicatesKeepRight(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for i := len(in) - 1; i >= 0; i-- {
		v := in[i]
		if seen[v] {
			continue
		}
		out = append([]string{v}, out...)
		seen[v] = true
	}

	return out
}
