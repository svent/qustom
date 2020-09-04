package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alexflint/go-arg"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

// JSParamType represents the (detected) type of a function parameter.
// TypeUndecidable is used when types could be inferred,
// but the result of e.g. multiple functions call is ambiguous.
type JSParamType int
type JSParamTypes []JSParamType

const (
	TypeUnknown JSParamType = iota
	TypeUndecidable
	TypeString
	TypeNumber
	TypeLong
	TypeHost
	TypePort
	TypeBoolean
)

// Config represents the content of a single configuration file.
type Config struct {
	Namespace    string
	Author       string
	Author_Email string
	Function     map[string]*Function
	Meta         toml.MetaData `toml:"-"`
}

// Namespace holds all functions associated to the used namespaces.
type Namespace struct {
	Functions map[string]Function
}

// Function stores function properties needed for generating the bundle.
type Function struct {
	Description      string
	ParameterTypes   string `toml:"parameter_types"`
	ReturnType       string `toml:"return_type"`
	VarArgs          bool   `toml:"var_args"`
	Source           string
	Includes         []string
	IncludesCompiled string `toml:"-"`
	Tests            []Test
}

// Test holds a test defined in a configuration file.
type Test struct {
	Call   string
	Expect interface{}
	Error  bool
	Null   bool
}

// JSParam stores the name and type of parsed JavaScript function parameters.
type JSParam struct {
	Name string
	Type JSParamType
}

// JSFunction stores detected properties of JavaScript functions.
type JSFunction struct {
	Name       string
	Params     []JSParam
	VarArgs    bool
	ReturnType JSParamType
}

// JSCall stores the detected properties of a parsed JavaScript test call.
type JSCall struct {
	Name       string
	ParamTypes []JSParamType
}

// CustomFunction is used to marshal the xml bundle.
type CustomFunction struct {
	Text                string `xml:",chardata"`
	Namespace           string `xml:"namespace"`
	Name                string `xml:"name"`
	ReturnType          string `xml:"return_type"`
	ParameterTypes      string `xml:"parameter_types"`
	ExecuteFunctionName string `xml:"execute_function_name"`
	ScriptEngine        string `xml:"script_engine"`
	Varargs             string `xml:"varargs"`
	Script              string `xml:"script"`
	Username            string `xml:"username"`
}

// Bundle holds a complete bundle with all defined custom functions.
type Bundle struct {
	XMLName        xml.Name         `xml:"content"`
	Text           string           `xml:",chardata"`
	CustomFunction []CustomFunction `xml:"custom_function"`
}

// GenerateCmd holds the parsed command line options for the generate command.
type GenerateCmd struct {
	Bundle string   `help:"path to bundle file"`
	Config []string `arg:"required,positional" help:"directory/file containing function definitions"`
}

// Arguments is used for the parsing of command line arguments.
type Arguments struct {
	Generate *GenerateCmd `arg:"subcommand:generate"`
}

var (
	errorLogger = log.New(os.Stderr, "Error: ", 0)
)

func (t JSParamType) String() string {
	switch t {
	case TypeString:
		return "String"
	case TypeNumber:
		return "Number"
	case TypeLong:
		return "Long"
	case TypeHost:
		return "Host"
	case TypePort:
		return "Port"
	case TypeBoolean:
		return "Boolean"
	default:
		return "UNKNOWN"
	}
}

// reflectJSParamType maps Go reflection types to JSParamType.
func reflectJSParamType(t reflect.Type) JSParamType {
	if t == nil {
		return TypeUnknown
	}
	switch t.Kind() {
	case reflect.String:
		return TypeString
	case reflect.Int, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		return TypeNumber
	case reflect.Bool:
		return TypeBoolean
	default:
		return TypeUnknown
	}
}

func (t JSParamTypes) allKnown() bool {
	for _, t := range t {
		if t == TypeUnknown {
			return false
		}
	}
	return true
}

func (t JSParamTypes) toStringSlice() []string {
	var res []string
	for _, t := range t {
		res = append(res, t.String())
	}
	return res
}

// updateType is used to consolidate the result of multiple
// type detections for the same parameter e.g. from multiple
// test calls.
func updateType(state *JSParamType, typ JSParamType) {
	if state == nil {
		return
	}
	if typ == TypeUnknown || *state == TypeUndecidable {
		return
	}
	if *state == TypeUnknown {
		*state = typ
		return
	}
	if typ != *state {
		*state = TypeUndecidable
	}
}

// analyzeParamTypes consolidates the result of multiple parsed
// and executed test calls to detect parameter and return types.
func analyzeParamTypes(types []JSParamTypes) (ret JSParamTypes, vararg bool) {
	varargtype := TypeUnknown
	if len(types) == 0 {
		return ret, false
	}
	ret = types[0]
	for _, t := range types[0] {
		updateType(&varargtype, t)
	}

	for _, ts := range types[1:] {
		if len(ts) != len(ret) {
			vararg = true
		}
		for i, _ := range ts {
			updateType(&varargtype, ts[i])
			if i < len(ret) {
				updateType(&ret[i], ts[i])
			}
		}
	}
	for i := range ret {
		if ret[i] == TypeUndecidable {
			ret[i] = TypeUnknown
		}
	}
	if varargtype == TypeUndecidable {
		varargtype = TypeUnknown
	}
	return ret, vararg
}

// processConfig parses a config file.
func processConfig(path string) (Config, error) {
	var config Config
	md, err := toml.DecodeFile(path, &config)
	if err != nil {
		return config, err
	}
	if len(md.Undecoded()) > 0 {
		for _, e := range md.Undecoded() {
			return config, fmt.Errorf("%s contains unknown key: %s\n", path, e)
		}
	}
	config.Meta = md
	return config, nil
}

// parseCall parses the JavaScript function call from a test definition.
func parseCall(name, src string) (JSCall, error) {
	program, err := parser.ParseFile(nil, name+".js", src, 0)
	if err != nil {
		return JSCall{}, fmt.Errorf("cannot parse source: %s", err)
	}
	body := program.Body
	for _, stmt := range body {
		exprStmt, ok := stmt.(*ast.ExpressionStatement)
		if !ok {
			continue
		}
		callexpr, ok := exprStmt.Expression.(*ast.CallExpression)
		if !ok {
			continue
		}
		id, ok := callexpr.Callee.(*ast.Identifier)
		if !ok || id.Name.String() != name {
			continue
		}
		var args []JSParamType
		for _, a := range callexpr.ArgumentList {
			switch a.(type) {
			case *ast.StringLiteral, *ast.Identifier, *ast.RegExpLiteral:
				args = append(args, TypeString)
			case *ast.NumberLiteral:
				args = append(args, TypeNumber)
			case *ast.BooleanLiteral:
				args = append(args, TypeBoolean)
			default:
				args = append(args, TypeUnknown)
			}
		}
		return JSCall{Name: id.Name.String(), ParamTypes: args}, nil
	}
	return JSCall{}, fmt.Errorf("no matching function call found")
}

// parseFunction parses a function from a source definition.
func parseFunction(name, src string) (JSFunction, error) {
	program, err := parser.ParseFile(nil, name+".js", src, 0)
	if err != nil {
		return JSFunction{}, fmt.Errorf("cannot parse source for function '%s': %s", name, err)
	}
	if len(program.DeclarationList) == 0 {
		return JSFunction{}, fmt.Errorf("no function declaration found")
	}

	// the source might contain helper functions, search the correct one
	var fd *ast.FunctionDeclaration
	for _, d := range program.DeclarationList {
		if f, ok := d.(*ast.FunctionDeclaration); ok && f.Function.Name.Name.String() == name {
			fd = f
			break
		}
	}
	if fd == nil {
		return JSFunction{}, fmt.Errorf("no function declaration named '%s' found", name)

	}

	fname := fd.Function.Name.Name
	var params []JSParam
	for _, p := range fd.Function.ParameterList.List {
		params = append(params, JSParam{Name: p.Name.String(), Type: TypeUnknown})
	}
	return JSFunction{Name: fname.String(), Params: params}, nil
}

// executeTest executes a test function together with the defined function
// using a JavaScript VM.
func executeTest(source, includes string, test Test) (JSParamType, error) {
	vm := goja.New()

	v, err := vm.RunString(includes)
	if err != nil {
		return TypeUnknown, fmt.Errorf("failed executing includes: %s", err)
	}

	v, err = vm.RunString(source)
	if err != nil {
		return TypeUnknown, fmt.Errorf("failed executing source code: %s", err)
	}

	v, err = vm.RunString(test.Call)
	if err != nil {
		if test.Error {
			return TypeUnknown, nil
		}
		return TypeUnknown, fmt.Errorf("failed executing test call: %s", err)
	}

	expect := fmt.Sprintf("%v", test.Expect)
	result := fmt.Sprintf("%v", v.Export())
	if test.Error {
		return TypeUnknown, fmt.Errorf("test call was supposed to fail but executed successfully")
	}
	if test.Null && v.Export() != nil {
		return TypeUnknown, fmt.Errorf("test call returned unexpected result: expected null, got '%v'", result)
	}
	if expect != result {
		return TypeUnknown, fmt.Errorf("test call returned unexpected result: expected '%v', got '%v'", expect, result)
	}

	return reflectJSParamType(v.ExportType()), nil
}

func compileIncludes(paths []string) (string, error) {
	var includes string
	for _, inc := range paths {
		files, err := ioutil.ReadDir(filepath.Join("includes", inc))
		if err != nil {
			return "", fmt.Errorf("failed reading include files: %s", err)
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".js") {
				continue
			}
			b, err := ioutil.ReadFile(filepath.Join("includes", inc, f.Name()))
			if err != nil {
				return "", fmt.Errorf("failed reading include file '%s': %s", f.Name(), err)
			}
			includes += string(b)
		}
	}
	return includes, nil
}

func generate(cmd *GenerateCmd) error {
	var configs []Config

	// parse configs
	for _, path := range cmd.Config {
		err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("cannot access %s: %w", path, err)
			}
			if !info.IsDir() {
				c, err := processConfig(path)
				if err != nil {
					return fmt.Errorf("cannot process config '%s': %s\n", path, err)
				}
				configs = append(configs, c)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	var bundle Bundle

	for _, config := range configs {
		// make sure we traverse (and thus error check etc.) the functions
		// always in the same order
		var fnames []string
		for k, _ := range config.Function {
			fnames = append(fnames, k)
		}
		sort.Strings(fnames)

		for _, fname := range fnames {
			fconf := config.Function[fname]
			jsf, err := parseFunction(fname, fconf.Source)
			if err != nil {
				return fmt.Errorf("cannot parse function '%s::%s': %s", config.Namespace, fname, err)
			}

			fconf.IncludesCompiled, err = compileIncludes(fconf.Includes)
			if err != nil {
				return fmt.Errorf("cannot process includes for function '%s::%s': %s", config.Namespace, fname, err)
			}

			var testParamTypes []JSParamTypes
			var retType JSParamType = TypeUnknown

			for i, t := range fconf.Tests {
				if t.Expect == nil && !t.Error && !t.Null {
					return fmt.Errorf("test call %d for function '%s::%s' does not specify at least one of the fields 'expect', 'error' or 'null'", i+1, config.Namespace, fname)
				}
				tc, err := parseCall(fname, t.Call)
				if err != nil {
					return fmt.Errorf("cannot parse test call %d for function '%s::%s': %w", i+1, config.Namespace, fname, err)
				}
				rt, err := executeTest(fconf.Source, fconf.IncludesCompiled, t)
				if err != nil {
					return fmt.Errorf("failed test %d for function '%s::%s': %s", i+1, config.Namespace, fname, err)
				}

				testParamTypes = append(testParamTypes, tc.ParamTypes)
				if rt != TypeUnknown {
					if retType == TypeUnknown {
						retType = rt
					} else {
						if rt != retType {
							retType = TypeUndecidable
						}
					}
				}
			}
			jsf.ReturnType = retType

			pt, vararg := analyzeParamTypes(testParamTypes)
			if len(jsf.Params) == len(pt) {
				for i := range jsf.Params {
					jsf.Params[i].Type = pt[i]
				}
			}
			jsf.VarArgs = vararg

			if !config.Meta.IsDefined("function", fname, "var_args") {
				fconf.VarArgs = jsf.VarArgs
			}

			if !config.Meta.IsDefined("function", fname, "parameter_types") {
				if pt.allKnown() {
					if len(pt) > 0 {
						if fconf.VarArgs {
							fconf.ParameterTypes = pt.toStringSlice()[0]
						} else {
							fconf.ParameterTypes = strings.Join(pt.toStringSlice(), " ")
						}
					}
				} else {
					return fmt.Errorf(`Parameter Types for function '%s::%s' could not be inferred, please specify them with 'parameter_types = "[String|Number|Long|Host|Port|Boolean] [...]"' or set 'var_args = true'`, config.Namespace, fname)
				}
			}

			if !config.Meta.IsDefined("function", fname, "return_type") {
				if jsf.ReturnType != TypeUnknown && jsf.ReturnType != TypeUndecidable {
					fconf.ReturnType = jsf.ReturnType.String()
				} else {
					return fmt.Errorf(`Return type for function '%s::%s' could not be inferred, please specify it with 'return_type = "[String|Number|Long|Host|Port|Boolean] [...]"' or add test functions`, config.Namespace, fname)
				}
			}

			paramtypes := strings.Split(fconf.ParameterTypes, " ")
			fsig := config.Namespace + "::" + fname
			var params []string
			for i, e := range jsf.Params {
				p := e.Name
				if i < len(paramtypes) {
					p += " " + paramtypes[i]
				}
				params = append(params, p)
			}
			fsig += "(" + strings.Join(params, ", ")
			if fconf.VarArgs {
				fsig += " [varargs]"
			}
			fsig += ")"
			fmt.Printf("Generated function %s => %s\n", fsig, fconf.ReturnType)
			var cf CustomFunction
			cf.Name = fname
			cf.ExecuteFunctionName = fname
			cf.Namespace = config.Namespace
			cf.ParameterTypes = fconf.ParameterTypes
			cf.ReturnType = fconf.ReturnType
			cf.Script = fconf.IncludesCompiled + fconf.Source
			cf.ScriptEngine = "javascript"
			cf.Username = "admin"
			cf.Varargs = fmt.Sprintf("%v", fconf.VarArgs)
			bundle.CustomFunction = append(bundle.CustomFunction, cf)
		}
	}

	if cmd.Bundle != "" {
		b, err := xml.MarshalIndent(&bundle, "", "  ")
		if err != nil {
			log.Fatalf("xml.MarshalIndent failed with '%s'\n", err)
		}
		err = ioutil.WriteFile(cmd.Bundle, b, 0644)
		if err != nil {
			log.Fatalf("failed writing bundle file '%s': %s", cmd.Bundle, err)
		}
		fmt.Printf("wrote bundle to %s.\n", cmd.Bundle)
	}

	return nil
}

func main() {
	var args Arguments
	p := arg.MustParse(&args)
	if p.Subcommand() == nil {
		p.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	switch {
	case args.Generate != nil:
		err := generate(args.Generate)
		if err != nil {
			errorLogger.Fatalln(err)
		}
	}
}
