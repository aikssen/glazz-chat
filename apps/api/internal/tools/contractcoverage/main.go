package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var generatedRoute = regexp.MustCompile(`\((GET|POST|PATCH|DELETE|PUT|OPTIONS|HEAD) ([^)]+)\)`)

type route struct {
	method string
	path   string
}

func main() {
	contract, err := contractRoutes("internal/platform/api/generated.gen.go")
	if err != nil {
		fail(err)
	}
	runtime, err := runtimeRoutes("internal/platform/server/server.go")
	if err != nil {
		fail(err)
	}

	allowedRuntimeOnly := map[route]string{
		{method: "GET", path: "/api/v1/ws"}:                  "AsyncAPI WebSocket handshake",
		{method: "GET", path: "/api/v1/auth/test/authorize"}: "development-only OAuth fixture",
	}
	var missing []route
	for item := range contract {
		if _, ok := runtime[item]; !ok {
			missing = append(missing, item)
		}
	}
	var undocumented []route
	for item := range runtime {
		if _, ok := contract[item]; ok {
			continue
		}
		if _, ok := allowedRuntimeOnly[item]; !ok {
			undocumented = append(undocumented, item)
		}
	}
	sortRoutes(missing)
	sortRoutes(undocumented)
	if len(missing) > 0 || len(undocumented) > 0 {
		for _, item := range missing {
			fmt.Fprintf(os.Stderr, "contract route missing at runtime: %s %s\n", item.method, item.path)
		}
		for _, item := range undocumented {
			fmt.Fprintf(os.Stderr, "runtime route missing from contract: %s %s\n", item.method, item.path)
		}
		os.Exit(1)
	}
	fmt.Printf(
		"contract coverage: %d OpenAPI routes, %d runtime routes, %d reviewed runtime-only routes\n",
		len(contract), len(runtime), len(allowedRuntimeOnly),
	)
}

func contractRoutes(filename string) (map[route]struct{}, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated contract: %w", err)
	}
	result := make(map[route]struct{})
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "ServerInterface" {
				continue
			}
			server, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				return nil, errors.New("generated ServerInterface is not an interface")
			}
			for _, method := range server.Methods.List {
				if method.Doc == nil {
					continue
				}
				match := generatedRoute.FindStringSubmatch(method.Doc.Text())
				if len(match) != 3 {
					continue
				}
				result[route{method: match[1], path: "/api/v1" + match[2]}] = struct{}{}
			}
		}
	}
	if len(result) == 0 {
		return nil, errors.New("generated ServerInterface contains no documented routes")
	}
	return result, nil
}

func runtimeRoutes(filename string) (map[route]struct{}, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse runtime server: %w", err)
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "New" || function.Body == nil {
			continue
		}
		result := make(map[route]struct{})
		walkRuntime(function.Body, "", result)
		return result, nil
	}
	return nil, errors.New("runtime server New function was not found")
}

func walkRuntime(node ast.Node, prefix string, result map[route]struct{}) {
	ast.Inspect(node, func(current ast.Node) bool {
		call, ok := current.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if selector.Sel.Name == "Route" && len(call.Args) >= 2 {
			path, pathOK := stringLiteral(call.Args[0])
			handler, handlerOK := call.Args[1].(*ast.FuncLit)
			if pathOK && handlerOK {
				walkRuntime(handler.Body, prefix+path, result)
				return false
			}
		}
		if selector.Sel.Name == "Group" && len(call.Args) >= 1 {
			if handler, ok := call.Args[0].(*ast.FuncLit); ok {
				walkRuntime(handler.Body, prefix, result)
				return false
			}
		}
		if !isHTTPMethod(selector.Sel.Name) || len(call.Args) == 0 {
			return true
		}
		path, ok := stringLiteral(call.Args[0])
		if ok {
			result[route{method: strings.ToUpper(selector.Sel.Name), path: prefix + path}] = struct{}{}
		}
		return true
	})
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func isHTTPMethod(value string) bool {
	switch value {
	case "Get", "Post", "Patch", "Delete", "Put", "Options", "Head":
		return true
	default:
		return false
	}
}

func sortRoutes(items []route) {
	sort.Slice(items, func(left, right int) bool {
		if items[left].path == items[right].path {
			return items[left].method < items[right].method
		}
		return items[left].path < items[right].path
	})
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
