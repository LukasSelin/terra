package terra

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// The passes' own source, read for the fields each touches: see
// TestTheCouplingsAreWhatThePassesTouch.
//
// It reads the package's Go with go/parser and no type checker, so what it
// knows of a name is what the code's shape says: g is a grid where a
// function takes a *Grid by that name, x.g and l.Grid are grids, and
// g.Height[i] = h writes Height. A write through a name the function took a
// field into (h := g.Height; h[i] = 0, or t := &g.Tiles[i]; t.Terrain = Ice)
// is the field's. What it cannot see - a field handed to another package
// that writes into it, a grid reached by a name it does not know as one - is
// what the run in TestTheStagesWriteWhatTheyDeclare catches, stage by stage.

// gridFields is every field a Grid has, by the name the code reads it by:
// its own, the embedded Layers' and Map's, and a Tile's as "Tiles.X".
var gridFields = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	var add func(t reflect.Type, prefix string)
	add = func(t reflect.Type, prefix string) {
		for f := range t.Fields() {
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				add(f.Type, prefix)
				continue
			}
			out[prefix+f.Name] = true
		}
	}
	add(reflect.TypeFor[Grid](), "")
	add(reflect.TypeFor[Tile](), "Tiles.")
	return out
})

// sourceFunc is one function of the package as the reading has it: the
// fields it reads and writes, and what it calls.
type sourceFunc struct {
	reads, writes map[string]bool
	calls         []string
}

// packageSource is every function of the package, by "Grid.drain" for a
// method and "drain" for a function.
type packageSource struct {
	funcs map[string]*sourceFunc
	// methods is every method's key by its bare name, whatever it is a
	// method of.
	methods map[string][]string
	// gridding is every function whose first result is a grid.
	gridding map[string]bool
	// mutates is every method's bare name that some method by that name
	// writes its own receiver under, through a pointer.
	mutates map[string]bool
}

var readSource = sync.OnceValues(func() (*packageSource, error) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	src := &packageSource{funcs: map[string]*sourceFunc{}, methods: map[string][]string{}}
	var decls []*ast.FuncDecl
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(fset, name, b, 0)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				decls = append(decls, fd)
			}
		}
	}
	src.gridding, src.mutates = map[string]bool{}, map[string]bool{}
	for _, fd := range decls {
		key := funcKey(fd)
		if fd.Recv != nil {
			src.methods[fd.Name.Name] = append(src.methods[fd.Name.Name], key)
		}
		if r := fd.Type.Results; r != nil && len(r.List) > 0 && typeName(r.List[0].Type) == "Grid" {
			src.gridding[key] = true
		}
		if mutating(fd) {
			src.mutates[fd.Name.Name] = true
		}
	}
	for _, fd := range decls {
		src.funcs[funcKey(fd)] = src.read(fd)
	}
	return src, nil
})

// funcKey is the key a function is kept under.
func funcKey(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return typeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

// typeName is the name of a receiver's or a parameter's type, without its
// star.
func typeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return typeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return typeName(t.X)
	}
	return ""
}

// read reads one function.
func (src *packageSource) read(fd *ast.FuncDecl) *sourceFunc {
	sf := &sourceFunc{reads: map[string]bool{}, writes: map[string]bool{}}
	grids, lands := map[string]bool{}, map[string]bool{}
	note := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				switch typeName(f.Type) {
				case "Grid":
					grids[n.Name] = true
				case "Land":
					lands[n.Name] = true
				}
			}
		}
	}
	note(fd.Recv)
	note(fd.Type.Params)
	fields := gridFields()
	// And the locals given a grid: by a function that makes or hands one
	// back, or declared as one.
	for range 2 {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for k, l := range x.Lhs {
					id, ok := l.(*ast.Ident)
					if !ok || len(x.Rhs) != len(x.Lhs) && k > 0 {
						continue
					}
					r := x.Rhs[min(k, len(x.Rhs)-1)]
					call, ok := r.(*ast.CallExpr)
					if !ok {
						continue
					}
					switch fn := call.Fun.(type) {
					case *ast.Ident:
						if src.gridding[fn.Name] {
							grids[id.Name] = true
						}
					case *ast.SelectorExpr:
						for _, m := range src.methods[fn.Sel.Name] {
							if src.gridding[m] && (!isIdent(fn.X, grids) || strings.HasPrefix(m, "Grid.")) {
								grids[id.Name] = true
							}
						}
					}
				}
			case *ast.ValueSpec:
				if typeName(x.Type) == "Grid" {
					for _, n := range x.Names {
						grids[n.Name] = true
					}
				}
			}
			return true
		})
	}

	// isGrid says e is a grid.
	isGrid := func(e ast.Expr) bool {
		switch x := e.(type) {
		case *ast.Ident:
			return grids[x.Name]
		case *ast.SelectorExpr:
			return x.Sel.Name == "g" || x.Sel.Name == "Grid"
		}
		return false
	}
	// field is the grid field e reads, as "Tiles.X" for a tile's, and the
	// selector it is read by; or nothing.
	parents := map[ast.Node]ast.Node{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		ast.Inspect(n, func(c ast.Node) bool {
			if c != nil && c != n {
				parents[c] = n
				return false
			}
			return c == n
		})
		return true
	})
	tileField := func(n ast.Node) string {
		// n is g.Tiles or an alias of it: the field of the tile it is
		// indexed for, if it is.
		ix, ok := parents[n].(*ast.IndexExpr)
		if !ok || ix.X != n {
			return ""
		}
		var up ast.Node = ix
		if p, ok := parents[ix].(*ast.ParenExpr); ok {
			up = p
		}
		if s, ok := parents[up].(*ast.SelectorExpr); ok && fields["Tiles."+s.Sel.Name] {
			return "Tiles." + s.Sel.Name
		}
		return ""
	}
	fieldOf := func(e ast.Expr) string {
		s, ok := e.(*ast.SelectorExpr)
		if !ok || !isGrid(s.X) || !fields[s.Sel.Name] {
			return ""
		}
		if s.Sel.Name == "Tiles" {
			if t := tileField(s); t != "" {
				return t
			}
		}
		return s.Sel.Name
	}
	// root is the expression an assignment to e writes into: e less its
	// indexing, slicing, dereferencing and fields of what it reaches.
	var root func(e ast.Expr) ast.Expr
	root = func(e ast.Expr) ast.Expr {
		if fieldOf(e) != "" {
			return e
		}
		switch x := e.(type) {
		case *ast.IndexExpr:
			return root(x.X)
		case *ast.SliceExpr:
			return root(x.X)
		case *ast.StarExpr:
			return root(x.X)
		case *ast.ParenExpr:
			return root(x.X)
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				return root(x.X)
			}
		case *ast.SelectorExpr:
			return root(x.X)
		}
		return e
	}
	// aliases is the field each local was taken from.
	aliases := map[string]string{}
	// given is the fields given something whole, which is not a reading of
	// them: g.rain = make(...), g.Height[i] = h.
	given := map[ast.Node]bool{}
	write := func(lhs ast.Expr, whole bool) {
		r := root(lhs)
		if f := fieldOf(r); f != "" {
			sf.writes[f] = true
			given[r] = given[r] || whole
			return
		}
		id, ok := r.(*ast.Ident)
		if !ok || id == lhs {
			return // the local itself, given something else
		}
		if f, ok := aliases[id.Name]; ok {
			if f == "Tiles" {
				if t := tileField(id); t != "" {
					f = t
				} else if s, ok := parents[id].(*ast.SelectorExpr); ok && fields["Tiles."+s.Sel.Name] {
					f = "Tiles." + s.Sel.Name // t := &g.Tiles[i]; t.Terrain = x
				}
			}
			sf.writes[f] = true
		}
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for k, l := range x.Lhs {
				write(l, x.Tok == token.ASSIGN || x.Tok == token.DEFINE)
				if len(x.Rhs) == len(x.Lhs) {
					if id, ok := l.(*ast.Ident); ok {
						if f := fieldOf(root(x.Rhs[k])); f != "" {
							aliases[id.Name] = f
						} else if r, ok := root(x.Rhs[k]).(*ast.Ident); ok && aliases[r.Name] != "" && r != x.Rhs[k] {
							aliases[id.Name] = aliases[r.Name]
						}
					}
				}
			}
		case *ast.IncDecStmt:
			write(x.X, false)
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "copy" || id.Name == "clear") && len(x.Args) > 0 {
				write(x.Args[0], true)
			}
		case *ast.SelectorExpr:
			if f := fieldOf(x); f != "" {
				if !given[x] {
					sf.reads[f] = true
				}
				return true
			}
			// A tile's field read through a local the tiles were taken
			// into, and a tile's own method, which reads what the tile is.
			if r := root(x.X); r != nil {
				if id, ok := r.(*ast.Ident); ok && aliases[id.Name] == "Tiles" {
					if fields["Tiles."+x.Sel.Name] {
						sf.reads["Tiles."+x.Sel.Name] = true
					} else if _, call := parents[x].(*ast.CallExpr); call {
						sf.reads["Tiles.Terrain"] = true
					}
				} else if fieldOf(r) == "Tiles" {
					if _, call := parents[x].(*ast.CallExpr); call && !fields["Tiles."+x.Sel.Name] {
						sf.reads["Tiles.Terrain"] = true
					}
				}
			}
			// A method that writes its receiver, called on a field of the
			// grid or a local taken from one, writes the field.
			if call, ok := parents[x].(*ast.CallExpr); ok && call.Fun == x && src.mutates[x.Sel.Name] {
				r := root(x.X)
				if f := fieldOf(r); f != "" {
					sf.writes[f] = true
				} else if id, ok := r.(*ast.Ident); ok && aliases[id.Name] != "" {
					sf.writes[aliases[id.Name]] = true
				}
			}
			// A method of a grid, a land, or of anything else by its name.
			name := x.Sel.Name
			switch {
			case isGrid(x.X):
				if _, ok := src.funcs["Grid."+name]; ok || slices.Contains(src.methods[name], "Grid."+name) {
					sf.calls = append(sf.calls, "Grid."+name)
				}
			case isIdent(x.X, lands):
				if slices.Contains(src.methods[name], "Land."+name) {
					sf.calls = append(sf.calls, "Land."+name)
				}
			default:
				// Only where it is called: a field of anything may share
				// a method's name.
				if call, ok := parents[x].(*ast.CallExpr); !ok || call.Fun != x {
					break
				}
				for _, k := range src.methods[name] {
					if !strings.HasPrefix(k, "Grid.") && !strings.HasPrefix(k, "Land.") {
						sf.calls = append(sf.calls, k)
					}
				}
			}
		case *ast.Ident:
			if _, isSel := parents[x].(*ast.SelectorExpr); !isSel || parents[x].(*ast.SelectorExpr).X == x {
				sf.calls = append(sf.calls, x.Name) // kept only where it names a function
			}
		}
		return true
	})
	return sf
}

// isIdent says e is a name among names.
func isIdent(e ast.Expr, names map[string]bool) bool {
	id, ok := e.(*ast.Ident)
	return ok && names[id.Name]
}

// touched is what the functions from reach, called down through the package
// until a function in stop: the fields read and written, and the functions
// in stop it came to.
func (src *packageSource) touched(from []string, stop map[string]bool) (reads, writes map[string]bool, nested []string) {
	reads, writes = map[string]bool{}, map[string]bool{}
	seen := map[string]bool{}
	var walk func(k string, top bool)
	walk = func(k string, top bool) {
		if seen[k] {
			return
		}
		sf, ok := src.funcs[k]
		if !ok {
			return
		}
		seen[k] = true
		if !top && stop[k] {
			nested = append(nested, k)
			return
		}
		for f := range sf.reads {
			reads[f] = true
		}
		for f := range sf.writes {
			writes[f] = true
		}
		for _, c := range sf.calls {
			walk(c, false)
		}
	}
	for _, k := range from {
		walk(k, true)
	}
	slices.Sort(nested)
	return reads, writes, nested
}

// mutating says fd is a method with a pointer receiver that writes under
// it: recv.x = y, recv[i]++, recv.x[i] += y.
func mutating(fd *ast.FuncDecl) bool {
	if fd.Recv == nil || len(fd.Recv.List) == 0 || len(fd.Recv.List[0].Names) == 0 {
		return false
	}
	if _, star := fd.Recv.List[0].Type.(*ast.StarExpr); !star {
		return false
	}
	recv := fd.Recv.List[0].Names[0].Name
	var under func(e ast.Expr) bool
	under = func(e ast.Expr) bool {
		switch x := e.(type) {
		case *ast.Ident:
			return x.Name == recv
		case *ast.SelectorExpr:
			return under(x.X)
		case *ast.IndexExpr:
			return under(x.X)
		case *ast.StarExpr:
			return under(x.X)
		case *ast.ParenExpr:
			return under(x.X)
		}
		return false
	}
	found := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if _, bare := l.(*ast.Ident); !bare && under(l) {
					found = true
				}
			}
		case *ast.IncDecStmt:
			if _, bare := x.X.(*ast.Ident); !bare && under(x.X) {
				found = true
			}
		}
		return !found
	})
	return found
}
