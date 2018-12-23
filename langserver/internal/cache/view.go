// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"
	"github.com/saibing/bingo/langserver/internal/source"
	"go/token"
	"sync"

	"golang.org/x/tools/go/packages"
)

type getLoadDirFunc func(filename string) string

type View struct {
	mu sync.Mutex // protects all mutable state of the view

	Config *packages.Config

	files map[source.URI]*File

	getLoadDir getLoadDirFunc
}

func NewView() *View {
	return &View{
		Config: &packages.Config{
			Mode:    packages.LoadAllSyntax,
			Fset:    token.NewFileSet(),
			Tests:   true,
			Overlay: make(map[string][]byte),
		},
		files: make(map[source.URI]*File),
	}
}

// HasParsed return true if package is not nil
func (v *View) HasParsed(uri source.URI) bool {
	v.mu.Lock()
	f, found := v.files[uri]
	v.mu.Unlock()
	if !found {
		return false
	}

	return f.pkg != nil
}

// GetFile returns a File for the given uri.
// It will always succeed, adding the file to the managed set if needed.
func (v *View) GetFile(uri source.URI) *File {
	v.mu.Lock()
	f := v.getFile(uri)
	v.mu.Unlock()
	return f
}

// getFile is the unlocked internal implementation of GetFile.
func (v *View) getFile(uri source.URI) *File {
	f, found := v.files[uri]
	if !found {
		f = &File{
			URI:  uri,
			view: v,
		}
		v.files[f.URI] = f
	}
	return f
}

func (v *View) parse(uri source.URI) error {
	path, err := uri.Filename()
	if err != nil {
		return err
	}
	v.Config.Dir = v.getLoadDir(path)
	pkgs, err := packages.Load(v.Config, fmt.Sprintf("file=%s", path))
	if len(pkgs) == 0 {
		if err == nil {
			err = fmt.Errorf("no packages found for %s", path)
		}
		return err
	}
	for _, pkg := range pkgs {
		// add everything we find to the files cache
		for _, fAST := range pkg.Syntax {
			// if a file was in multiple packages, which token/ast/pkg do we store
			fToken := v.Config.Fset.File(fAST.Pos())
			fURI := source.ToURI(fToken.Name())
			f := v.getFile(fURI)
			f.token = fToken
			f.ast = fAST
			f.pkg = pkg
		}
	}
	return nil
}
