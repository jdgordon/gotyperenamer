package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/urfave/cli"
	"golang.org/x/tools/go/ast/astutil"
)

func identName(n ast.Expr) string {
	if x, ok := n.(*ast.Ident); ok && x != nil {
		return x.Name
	}
	return ""
}

func (f *fixer) applyFunc(c *astutil.Cursor) bool {
	switch n := c.Node().(type) {
	case *ast.SelectorExpr:
		switch c.Parent().(type) {
		case *ast.StarExpr, *ast.Field, *ast.CompositeLit:
			if ok, target := f.isReplaceTarget(n); ok {
				f.needsAdd = true
				c.Replace(&target)
				return false
			}
		}
	case *ast.File:
		f.pkg = identName(n.Name)
	case *ast.TypeSpec:
		if ok, _ := f.isReplaceTarget(n); ok {
			return false
		}
	case *ast.CallExpr:
		if ok, target := f.isReplaceTarget(n.Fun); ok {
			n.Fun = &target
			f.needsAdd = true
			c.Replace(n)
		}
	case *ast.Ident:
		if ok, target := f.isReplaceTarget(n); ok {
			switch c.Parent().(type) {
			case *ast.TypeSpec, *ast.FuncDecl, *ast.SelectorExpr:
			// nothing
			default:
				f.needsAdd = true
				c.Replace(&target)
			}
		}
	}

	return true
}

type fixer struct {
	importLine string
	pkg        string
	repls      []replData
	needsAdd   bool
}

func (f fixer) Fix(fset *token.FileSet, tree *ast.File) *ast.File {
	f.pkg = ""
	f.needsAdd = false

	result := astutil.Apply(tree, f.applyFunc, nil)
	if f.needsAdd {
		astutil.AddImport(fset, tree, f.importLine)
	}
	return result.(*ast.File)
}

func (f fixer) isReplaceTarget(n interface{}) (bool, ast.SelectorExpr) {
	for _, r := range f.repls {
		if r.isReplaceTarget(f.pkg, n) {
			return true, r.to
		}
	}
	return false, ast.SelectorExpr{}
}

type replData struct {
	from string
	to   ast.SelectorExpr
}

func (r replData) isReplaceTarget(pkg string, n interface{}) bool {
	ident := ""
	switch n := n.(type) {
	case ast.Ident:
		ident = n.Name
	case *ast.Ident:
		ident = n.Name
	case *ast.SelectorExpr:
		pkg = identName(n.X)
		ident = n.Sel.Name
	default:
		return false
	}
	return r.from == fmt.Sprintf("%s.%s", pkg, ident)
}

func (r replData) pkgName() string {
	return strings.Split(r.from, ".")[0]
}

func newReplData(val string) replData {
	parts := strings.Split(val, ":")
	if len(parts) != 2 {
		panic("invalid repl value: " + val)
	}
	dest := strings.Split(parts[1], ".")
	return replData{
		from: parts[0],
		to: ast.SelectorExpr{
			X:   ast.NewIdent(dest[0]),
			Sel: ast.NewIdent(dest[1]),
		},
	}
}

func fixFile(c *cli.Context, repls []replData, filename string) error {
	text, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, text, parser.ParseComments)
	if err != nil {
		return err
	}

	ff := fixer{
		importLine: c.String("import"),
		repls:      repls,
	}
	f = ff.Fix(fset, f)
	for _, imp := range f.Imports {
		for _, r := range repls {
			pkg := r.pkgName()
			impPath, _ := strconv.Unquote(imp.Path.Value)
			if (imp.Name != nil && pkg == identName(imp.Name)) || (pkg == filepath.Base(impPath)) {
				if !astutil.UsesImport(f, impPath) {
					_ = astutil.DeleteImport(fset, f, impPath) ||
						astutil.DeleteNamedImport(fset, f, pkg, impPath)
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return err
	}
	if c.IsSet("inplace") {
		ioutil.WriteFile(filename, buf.Bytes(), 0644)
	} else {
		fmt.Print(buf.String())
	}
	return nil
}

func fix(c *cli.Context) error {
	var repls []replData
	for _, repl := range c.StringSlice("replace") {
		repls = append(repls, newReplData(repl))
	}

	if c.IsSet("dir") {
		err := filepath.Walk(c.String("dir"), func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				if err == nil && filepath.Ext(path) == ".go" {
					return fixFile(c, repls, path)
				}
			}
			return nil
		})
		return err
	}

	for _, filename := range c.Args() {
		if err := fixFile(c, repls, filename); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "go typereplacer"

	app.Flags = []cli.Flag{
		&cli.StringSliceFlag{
			Name:     "replace",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "import",
			Required: true,
		},
		&cli.StringFlag{
			Name: "dir",
		},
		&cli.BoolFlag{
			Name: "inplace",
		},
	}
	app.Action = fix

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
