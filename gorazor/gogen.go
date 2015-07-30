package gorazor

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/fsnotify.v1"
	"go/ast"
	"go/printer"
	"bytes"
)

var GorazorNamespace = `"github.com/sipin/gorazor/gorazor"`

var cfg = printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}


//------------------------------ Compiler ------------------------------ //
const (
	CMKP = iota
	CBLK
	CSTAT
)

func getValStr(e interface{}) string {
	switch v := e.(type) {
	case *Ast:
		return v.TagName
	case Token:
		if !(v.Type == AT || v.Type == AT_COLON) {
			return v.Text
		}
		return ""
	default:
		panic(e)
	}
}

type Part struct {
	ptype int
	value string
}

type Param struct {
	Name string
	Type ast.Spec
}

type Compiler struct {
	ast      *Ast
	buf      string //the final result
	layout   string
	firstBLK int
	params   []Param
	parts    []Part
	imports  map[string]bool
	options  Option
	dir      string
	file     string
}

func (self *Compiler) addPart(part Part) {
	if len(self.parts) == 0 {
		self.parts = append(self.parts, part)
		return
	}
	last := &self.parts[len(self.parts)-1]
	if last.ptype == part.ptype {
		last.value += part.value
	} else {
		self.parts = append(self.parts, part)
	}
}

func (self *Compiler) genPart() {
	res := ""

	for _, p := range self.parts {
		if p.ptype == CMKP && p.value != "" {
			// do some escapings
			for strings.HasSuffix(p.value, "\n") {
				p.value = p.value[:len(p.value)-1]
			}
			if p.value != "" {
				p.value = fmt.Sprintf("%#v", p.value)
				res += "io.WriteString(w, " + p.value + ")\n"
			}
		} else if p.ptype == CBLK {
			res += p.value + "\n"
		} else {
			res += p.value
		}
	}
	self.buf = res
}

func makeCompiler(ast *Ast, options Option, input string) *Compiler {
	dir := filepath.Base(filepath.Dir(input))
	file := strings.Replace(filepath.Base(input), gz_extension, "", 1)
	if options["NameNotChange"] == nil {
		file = Capitalize(file)
	}
	return &Compiler{ast: ast, buf: "",
		layout: "", firstBLK: 0,
		params: []Param{},
		parts: []Part{},
		imports: map[string]bool{},
		options: options,
		dir:     dir,
		file:    file,
	}
}

func (cp *Compiler) visitBLK(child interface{}, ast *Ast) {
	cp.addPart(Part{CBLK, getValStr(child)})
}

func (cp *Compiler) visitMKP(child interface{}, ast *Ast) {

	cp.addPart(Part{CMKP, getValStr(child)})
}

// First block contains imports and parameters, specific action for layout,
// NOTE, layout have some conventions.
func (cp *Compiler) visitFirstBLK(blk *Ast) {
	pre := cp.buf
	cp.buf = ""
	first := ""
	backup := cp.parts
	cp.parts = []Part{}
	cp.visitAst(blk)
	cp.genPart()
	first, cp.buf = cp.buf, pre
	cp.parts = backup

	log.Println("First: ", first)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", "package main\n"+first, parser.DeclarationErrors)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		for _, s := range f.Imports {
			v := s.Path.Value
			if s.Name != nil {
				v = s.Name.Name + " " + v
			}
			parts := strings.SplitN(v, "/", -1)
			if len(parts) >= 2 && parts[len(parts)-2] == "layout" {
				cp.layout = strings.Replace(v, "\"", "", -1)
				dir := strings.Join(parts[0:len(parts)-1], "/") + "\""
				cp.imports[dir] = true
			} else {
				cp.imports[v] = true
			}
		}

		for _, dec := range f.Decls {
			if dec, ok := dec.(*ast.GenDecl); ok {
				log.Printf("Dec: %+v\n", dec)
				for _, spec := range dec.Specs {
					if spec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range spec.Names {
							cp.params = append(cp.params, Param{Name: name.Name, Type: spec})
						}
					}
				}
			}
		}
	}

	if cp.layout != "" {
		// Fix the path before looking for the gohtml file; we may not be in the same dir.
		incdir_abs, _ := cp.options["InDirAbs"].(string)
		outdir_abs, _ := cp.options["OutDirAbs"].(string)

		path := os.ExpandEnv("$GOPATH/src/" + cp.layout + ".gohtml")
		if incdir_abs != "" && outdir_abs != "" {
			path = strings.Replace(path, outdir_abs, incdir_abs, -1)
		}

		if exists(path) && len(LayOutArgs(path)) == 0 {
			//TODO, bad for performance
			_cp, err := run(path, cp.options)
			if err != nil {
				panic(err)
			}
			SetLayout(cp.layout, _cp.params)
		}
	}
}

func (cp *Compiler) visitExp(child interface{}, parent *Ast, idx int, isHomo bool) {
	start := ""
	end := ""
	ppNotExp := true
	ppChildCnt := len(parent.Children)
	pack := cp.dir
	htmlEsc := cp.options["htmlEscape"]
	if parent.Parent != nil && parent.Parent.Mode == EXP {
		ppNotExp = false
	}
	val := getValStr(child)
	needEscape := false
	writeableExp := false
	if htmlEsc == nil {
		if ppNotExp && idx == 0 && isHomo {
			needEscape = true
			switch {
			case val == "helper" || val == "html" || val == "raw":
				needEscape = false
			case pack == "layout":
				needEscape = true
				for _, param := range cp.params {
					if param.Name == val {
						needEscape = false
						if param, ok := param.Type.(*ast.ValueSpec); ok {
							if exp, ok := param.Type.(*ast.SelectorExpr); ok {
								if exp.Sel != nil && exp.Sel.Name == "Section" {
									if x, ok := exp.X.(*ast.Ident); ok && x.Name == "gorazor" {
										writeableExp = true
									}
								}
							}
						}
						break
					}
				}
			}
		}
		if ppNotExp && idx == ppChildCnt-1 && isHomo {
			end += ")"
		}
	}

	if ppNotExp && idx == 0 {
		if needEscape {
			if writeableExp {
				start = "gorazor.HTMLEscapeWriter(w, (" + start
			} else {
				start = "gorazor.HTMLEscapeWriter(w, (" + start
			}
			cp.imports[GorazorNamespace] = true
		} else {
			if writeableExp {
				start = "("
				end += "(w"
			} else {
				start = "io.WriteString(w, (" + start
			}
		}
	}
	if ppNotExp && idx == ppChildCnt-1 {
		end += ")\n"
	}

	v := start
	if val == "raw" {
		v += end
	} else {
		v += val + end
	}
	cp.addPart(Part{CSTAT, v})
}

func (cp *Compiler) visitAst(ast *Ast) {
	switch ast.Mode {
	case MKP:
		cp.firstBLK = 1
		for _, c := range ast.Children {
			if _, ok := c.(Token); ok {
				cp.visitMKP(c, ast)
			} else {
				cp.visitAst(c.(*Ast))
			}
		}
	case BLK:
		if cp.firstBLK == 0 {
			cp.firstBLK = 1
			cp.visitFirstBLK(ast)
		} else {
			remove := false
			if len(ast.Children) >= 2 {
				first := ast.Children[0]
				last := ast.Children[len(ast.Children)-1]
				v1, ok1 := first.(Token)
				v2, ok2 := last.(Token)
				if ok1 && ok2 && v1.Text == "{" && v2.Text == "}" {
					remove = true
				}
			}
			for idx, c := range ast.Children {
				if remove && (idx == 0 || idx == len(ast.Children)-1) {
					continue
				}
				if _, ok := c.(Token); ok {
					cp.visitBLK(c, ast)
				} else {
					cp.visitAst(c.(*Ast))
				}
			}
		}
	case EXP:
		cp.firstBLK = 1
		nonExp := ast.hasNonExp()
		for i, c := range ast.Children {
			if _, ok := c.(Token); ok {
				cp.visitExp(c, ast, i, !nonExp)
			} else {
				cp.visitAst(c.(*Ast))
			}
		}
	case PRG:
		for _, c := range ast.Children {
			cp.visitAst(c.(*Ast))
		}
	}
}

func (cp *Compiler) processSections() map[string][]string {
	lines := strings.SplitN(cp.buf, "\n", -1)
	sections := map[string][]string{}
	body := []string{}
	var currentSection []string
	var currentSectionName string
	//currentSection = append(currentSection, "body := func(w io.Writer) {\n")
	scope := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "section") && strings.HasSuffix(l, "{") {
			name := l
			name = strings.TrimSpace(name[7 : len(name)-1])
			scope = 1
			currentSection = []string{}
			currentSectionName = name
		} else if scope > 0 {
			if strings.HasSuffix(l, "{") {
				scope++
			} else if strings.HasSuffix(l, "}") {
				scope--
			}
			if scope == 0 {
				sections[currentSectionName] = currentSection
				scope = 0
			} else {
				currentSection = append(currentSection, l + "\n")
			}
		} else {
			body = append(body, l + "\n")
		}
	}
	sections["body"] = body
	return sections
}

// TODO, this is dirty now
func (cp *Compiler) processLayout(sections map[string][]string) {
	var out bytes.Buffer
	if (len(sections) == 1) {
		for _, line := range sections["body"] {
			out.WriteString(line)
		}
	} else {
		for name, section := range sections {
			out.WriteString(name + " := func(w io.Writer) {\n")
			for _, line := range section {
				out.WriteString(line)
			}
			out.WriteString("}\n")
		}
	}
	cp.buf = out.String()
	foot := ""
	if cp.layout != "" {
		parts := strings.SplitN(cp.layout, "/", -1)
		base := Capitalize(parts[len(parts)-1])
		foot += "layout.Write" + base + "(w"
		args := LayOutArgs(cp.layout)
		if len(args) == 0 {
			for name, _ := range sections {
				foot += ", " + name
			}
		} else {
			for _, param := range args {
				arg := param.Name
				found := false
				for sec, _ := range sections {
					if sec == arg {
						found = true
						foot += ", " + sec
						break
					}
				}
				if !found {
					for _, p := range cp.params {
						if p.Name == arg {
							found = true
							foot += ", " + p.Name
							break
						}
					}
				}
				if !found {
					foot += ", " + `""`
				}
			}
		}
		foot += ")"
	}

	foot += "\n}\n"
	cp.buf += foot
}

func (cp *Compiler) visit() {
	cp.visitAst(cp.ast)
	cp.genPart()

	pack := cp.dir
	fun := cp.file

	cp.imports[`"bytes"`] = true
	cp.imports[`"io"`] = true

	var head bytes.Buffer

	head.WriteString("package ")
	head.WriteString(pack)
	head.WriteString("\n import (\n")

	for k, _ := range cp.imports {
		head.WriteString(k)
		head.WriteByte('\n')
	}

	head.WriteString("\n)\n func ")
	head.WriteString(fun)
	head.WriteString("(")
	for idx, p := range cp.params {
		if idx > 0 {
			head.WriteString(", ")
		}
		cfg.Fprint(&head, token.NewFileSet(), p.Type)
	}
	head.WriteString(") string {\n")
	head.WriteString("\tvar _buffer bytes.Buffer\n")
	head.WriteString("\tWrite")
	head.WriteString(fun)
	head.WriteString("(&_buffer")

	for _, p := range cp.params {
		head.WriteString(", ")
		head.WriteString(p.Name)
	}
	head.WriteString(")\n")
	head.WriteString("\treturn _buffer.String()\n")
	head.WriteString("}\n\n")
	head.WriteString("func Write")
	head.WriteString(fun)
	head.WriteString("(w io.Writer")
	for _, p := range cp.params {
		head.WriteString(", ")
		cfg.Fprint(&head, token.NewFileSet(), p.Type)
	}
	head.WriteString(") {\n")
	log.Println("Visit cp.buf", cp.buf)
	sections := cp.processSections()
	cp.processLayout(sections)
	cp.buf = head.String() + cp.buf
}

func run(path string, Options Option) (*Compiler, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(content)
	lex := &Lexer{text, Tests}

	res, err := lex.Scan()
	if err != nil {
		return nil, err
	}

	//DEBUG
	if Options["Debug"] != nil {
		fmt.Println("------------------- TOKEN START -----------------")
		for _, elem := range res {
			elem.P()
		}
		fmt.Println("--------------------- TOKEN END -----------------\n")
	}

	parser := &Parser{&Ast{}, nil, res, []Token{}, false, UNK}
	err = parser.Run()
	if err != nil {
		fmt.Println(path, ":", err)
		os.Exit(2)
	}

	//DEBUG
	if Options["Debug"] != nil {
		fmt.Println("--------------------- AST START -----------------")
		parser.ast.debug(0, 20)
		fmt.Println("--------------------- AST END -----------------\n")
		if parser.ast.Mode != PRG {
			panic("TYPE")
		}
	}
	cp := makeCompiler(parser.ast, Options, path)
	cp.visit()
	return cp, nil
}

func generate(path string, output string, Options Option) error {
	cp, err := run(path, Options)
	if err != nil || cp == nil {
		panic(err)
	}
	err = ioutil.WriteFile(output, []byte(cp.buf), 0644)
	cmd := exec.Command("gofmt", "-w", output)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("gofmt: ", err)
		return err
	}
	if Options["Debug"] != nil {
		content, _ := ioutil.ReadFile(output)
		fmt.Println(string(content))
	}
	return err
}

func watchDir(input, output string, options Option) error {
	log.Println("Watching dir:", input, output)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)

	output_path := func(path string) string {
		res := strings.Replace(path, input, output, 1)
		return res
	}

	gen := func(filename string) error {
		outpath := output_path(filename)
		outpath = strings.Replace(outpath, ".gohtml", ".go", 1)
		outdir := filepath.Dir(outpath)
		if !exists(outdir) {
			os.MkdirAll(outdir, 0775)
		}
		err := GenFile(filename, outpath, options)
		if err == nil {
			log.Printf("%s -> %s\n", filename, outpath)
		}
		return err
	}

	visit_gen := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			//Just do file with exstension .gohtml
			if !strings.HasSuffix(path, ".gohtml") {
				return nil
			}
			filename := filepath.Base(path)
			if strings.HasPrefix(filename, ".#") {
				return nil
			}
			err := gen(path)
			if err != nil {
				return err
			}
		}
		return nil
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				filename := event.Name
				if filename == "" {
					//should be a bug for fsnotify
					continue
				}
				if event.Op&fsnotify.Remove != fsnotify.Remove &&
					(event.Op&fsnotify.Write == fsnotify.Write ||
						event.Op&fsnotify.Create == fsnotify.Create) {
					stat, err := os.Stat(filename)
					if err != nil {
						continue
					}
					if stat.IsDir() {
						log.Println("add dir:", filename)
						watcher.Add(filename)
						output := output_path(filename)
						log.Println("mkdir:", output)
						if !exists(output) {
							os.MkdirAll(output, 0755)
							err = filepath.Walk(filename, visit_gen)
							if err != nil {
								done <- true
							}
						}
						continue
					}
					if !strings.HasPrefix(filepath.Base(filename), ".#") &&
						strings.HasSuffix(filename, ".gohtml") {
						gen(filename)
					}
				} else if event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					output := output_path(filename)
					if exists(output) {
						//shoud be dir
						watcher.Remove(filename)
						os.RemoveAll(output)
						log.Println("remove dir:", output)
					} else if strings.HasSuffix(output, ".gohtml") {
						output = strings.Replace(output, ".gohtml", ".go", 1)
						if exists(output) {
							os.Remove(output)
							log.Println("removing file:", output)
						}
					}
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
				continue
			}
		}
	}()

	visit := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			watcher.Add(path)
		}
		return nil
	}

	err = filepath.Walk(input, visit)
	err = watcher.Add(input)
	if err != nil {
		log.Fatal(err)
	}
	<-done
	return nil
}
