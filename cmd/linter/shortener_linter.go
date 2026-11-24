package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var ShortenerLinter = &analysis.Analyzer{
	Name: "shortenerlinter",
	Doc:  `Report "panic" calls existence and report calls to the "log.Fatal" and/or "os.Exit" functions outside the main function of the main package`,
	Run:  run,
}

// запрещённые вне main.main функции: пакет -> имена
var exitFuncs = map[string]map[string]bool{
	"log": {"Fatal": true, "Fatalf": true, "Fatalln": true},
	"os":  {"Exit": true},
}

func run(pass *analysis.Pass) (interface{}, error) {
	isMainPkg := pass.Pkg.Name() == "main"

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			// внутри функции main пакета main log.Fatal и os.Exit разрешены
			inMain := false
			if fn, ok := decl.(*ast.FuncDecl); ok {
				inMain = isMainPkg && fn.Recv == nil && fn.Name.Name == "main"
			}

			ast.Inspect(decl, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					checkCall(pass, call, inMain)
				}
				return true
			})
		}
	}

	return nil, nil
}

func checkCall(pass *analysis.Pass, call *ast.CallExpr, inMain bool) {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		// встроенная функция panic (а не пользовательская с таким же именем)
		if b, ok := pass.TypesInfo.Uses[fn].(*types.Builtin); ok && b.Name() == "panic" {
			pass.Reportf(call.Pos(), "use of built-in panic function")
		}
	case *ast.SelectorExpr:
		if inMain {
			return
		}
		f, ok := pass.TypesInfo.Uses[fn.Sel].(*types.Func)
		if !ok || f.Pkg() == nil {
			return
		}
		if names, ok := exitFuncs[f.Pkg().Path()]; ok && names[f.Name()] {
			pass.Reportf(call.Pos(), "call to %s.%s outside of main function of main package", f.Pkg().Name(), f.Name())
		}
	}
}
