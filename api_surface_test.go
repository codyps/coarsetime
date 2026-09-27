package coarsetime

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Keep platform implementations private and the public declarations shared.
// This checks selected source files even for targets we cannot execute here.
func TestPortableAPISurface(t *testing.T) {
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		t.Skip("source checkout required for API surface inspection")
	}
	targets := []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "linux/386", "linux/arm", "linux/riscv64", "windows/amd64", "windows/arm64", "windows/386", "freebsd/amd64", "js/wasm"}
	want := []string{"Instant", "Instant.After", "Instant.Before", "Instant.Sub", "Now", "NowInstant", "Since", "UnixNano"}
	for _, target := range targets {
		for _, pure := range []bool{false, true} {
			name := target
			if pure {
				name += "/purego"
			}
			t.Run(name, func(t *testing.T) {
				ctx := build.Default
				parts := strings.Split(target, "/")
				ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = parts[0], parts[1], false
				ctx.BuildTags = nil
				if pure {
					ctx.BuildTags = []string{"purego"}
				}
				pkg, err := ctx.ImportDir(".", 0)
				if err != nil {
					t.Fatal(err)
				}
				if pure && (len(pkg.SFiles) != 0 || len(pkg.CgoFiles) != 0) {
					t.Fatalf("purego selected native sources: assembly=%v cgo=%v", pkg.SFiles, pkg.CgoFiles)
				}
				var got []string
				for _, file := range pkg.GoFiles {
					f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments)
					if err != nil {
						t.Fatal(err)
					}
					if pure {
						for _, imp := range f.Imports {
							if imp.Path.Value == `"unsafe"` || imp.Path.Value == `"syscall"` {
								t.Errorf("purego selected native access import %s in %s", imp.Path.Value, file)
							}
						}
						for _, group := range f.Comments {
							for _, comment := range group.List {
								if strings.HasPrefix(comment.Text, "//go:linkname") || strings.HasPrefix(comment.Text, "//go:cgo_") {
									t.Errorf("purego selected native bridge directive in %s: %s", file, comment.Text)
								}
							}
						}
					}
					add := func(name string) {
						got = append(got, name)
						if file != "instant.go" && file != "wall.go" {
							t.Errorf("public declaration %s in platform implementation %s", name, file)
						}
					}
					for _, d := range f.Decls {
						switch d := d.(type) {
						case *ast.FuncDecl:
							if !d.Name.IsExported() {
								continue
							}
							name := d.Name.Name
							if d.Recv != nil {
								receiver := d.Recv.List[0].Type
								if p, ok := receiver.(*ast.StarExpr); ok {
									receiver = p.X
								}
								name = receiver.(*ast.Ident).Name + "." + name
							}
							add(name)
						case *ast.GenDecl:
							for _, spec := range d.Specs {
								switch s := spec.(type) {
								case *ast.TypeSpec:
									if s.Name.IsExported() {
										add(s.Name.Name)
									}
								case *ast.ValueSpec:
									for _, n := range s.Names {
										if n.IsExported() {
											add(n.Name)
										}
									}
								}
							}
						}
					}
				}
				sort.Strings(got)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("API = %v, want %v", got, want)
				}
			})
		}
	}
}
