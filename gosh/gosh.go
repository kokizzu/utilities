// gosh
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nickwells/param.mod/v5/param"
	"github.com/nickwells/xdg.mod/xdg"
)

const (
	dfltHTTPPort        = 8080
	dfltHTTPPath        = "/"
	dfltHTTPHandlerName = "goshHandler"

	dfltSplitPattern = `\s+`

	dfltFormatter      = "gofmt"
	dfltFormatterArg   = "-w"
	goImportsFormatter = "goimports"

	goshCommentIntro = " gosh : "
)

// Gosh records all the details needed to build a Gosh program
type Gosh struct {
	w           *os.File
	indent      int
	addComments bool

	script       []string
	beforeScript []string
	afterScript  []string
	globalsList  []string
	imports      []string

	runInReadLoop bool
	inPlaceEdit   bool
	splitLine     bool
	splitPattern  string

	runAsWebserver bool
	httpHandler    string
	httpPort       int64
	httpPath       string

	runInReadloopSetters  []*param.ByName
	runAsWebserverSetters []*param.ByName

	showFilename  bool
	dontClearFile bool
	dontRun       bool
	filename      string
	cleanupPath   string

	formatter     string
	formatterSet  bool
	formatterArgs []string

	filesToRead []string
	filesErrMap param.ErrMap

	snippetsDirs []string
	showSnippets bool

	baseTempDir string
}

// NewGosh creates a new instance of the Gosh struct with all the initial
// default values set correctly.
func NewGosh() *Gosh {
	g := &Gosh{
		addComments:  true,
		splitPattern: dfltSplitPattern,
		formatter:    dfltFormatter,
		filesErrMap:  make(param.ErrMap),

		httpPort:    dfltHTTPPort,
		httpPath:    dfltHTTPPath,
		httpHandler: dfltHTTPHandlerName,
	}
	g.formatterArgs = append(g.formatterArgs, dfltFormatterArg)

	snippetPath := []string{
		"github.com",
		"nickwells",
		"utilities",
		"gosh",
		"snippets"}

	g.snippetsDirs = []string{
		filepath.Join(append([]string{xdg.ConfigHome()}, snippetPath...)...),
	}
	dirs := xdg.ConfigDirs()
	if len(dirs) > 0 {
		g.snippetsDirs = append(g.snippetsDirs,
			filepath.Join(append(dirs[:1], snippetPath...)...))
	}

	return g
}

// in increases the indent level by 1
func (g *Gosh) in() {
	g.indent++
}

// out decreases the indent level by 1
func (g *Gosh) out() {
	g.indent--
}

// indentStr returns a string to provide the current indent
func (g *Gosh) indentStr() string {
	return strings.Repeat("\t", g.indent)
}

// comment returns the standard comment string explaining why the line is
// in the generated code
func (g *Gosh) comment(text string) string {
	if !g.addComments {
		return ""
	}
	return "\t//" + goshCommentIntro + text
}

// varInfo records information about a variable. This is for the
// autogenerated variable declarations and for generating the note for the
// usage message
type varInfo struct {
	typeName string
	desc     string
}
type varMap map[string]varInfo

var knownVarMap varMap = varMap{
	"_r": {
		typeName: "io.Reader",
		desc:     "the reader for the scanner (may be stdin)",
	},
	"_rw": {
		typeName: "http.ResponseWriter",
		desc:     "the response writer for the web server",
	},
	"_req": {
		typeName: "*http.Request",
		desc:     "the request to the seb server",
	},
	"_gh": {
		typeName: dfltHTTPHandlerName,
		desc:     "the HTTP Handler (providing ServeHTTP)",
	},
	"_w": {
		typeName: "*os.File",
		desc:     "the file written to if editing in place",
	},
	"_l": {
		typeName: "*bufio.Scanner",
		desc:     "a buffered scanner used to read the files",
	},
	"_fn": {
		typeName: "string",
		desc:     "the name of the file (or stdin)",
	},
	"_fns": {
		typeName: "[]string",
		desc:     "the list of names of the files",
	},
	"_f": {
		typeName: "*os.File",
		desc:     "the file being read",
	},
	"_err": {
		typeName: "error",
		desc:     "an error",
	},
	"_sre": {
		typeName: "*regexp.Regexp",
		desc:     "the regexp used to split lines",
	},
	"_lp": {
		typeName: "[]string",
		desc:     "the parts of the line (when split)",
	},
}

// nameType looks up the name in knownVarMap and if it is found it will
// return the name and type as a single string suitable for use as a variable
// or parameter declaration
func (g *Gosh) nameType(name string) string {
	vi, ok := knownVarMap[name]
	if !ok {
		panic(fmt.Errorf("%q is not in the map of known variables", name))
	}
	return name + " " + vi.typeName
}

// gDecl declares a variable. The variable must be in the map of known
// variables (which is used to provide a note for the usage message). The
// declaration is indented and the Gosh comment is added
func (g *Gosh) gDecl(name, initVal, tag string) {
	fmt.Fprintln(g.w,
		g.indentStr()+"var "+g.nameType(name)+initVal+g.comment(tag))
}

// makeKnownVarList will format the entries in knownVarMap into a form
// suitable for the usage message
func makeKnownVarList() string {
	kvl := ""
	var keys = make([]string, 0, len(knownVarMap))
	maxVarNameLen := 0
	maxTypeNameLen := 0
	for k, vi := range knownVarMap {
		keys = append(keys, k)
		if len(k) > maxVarNameLen {
			maxVarNameLen = len(k)
		}
		if len(vi.typeName) > maxTypeNameLen {
			maxTypeNameLen = len(vi.typeName)
		}
	}
	sort.Strings(keys)

	sep := ""
	for _, k := range keys {
		vi := knownVarMap[k]
		kvl += fmt.Sprintf("%s%-*.*s %-*.*s  %s",
			sep,
			maxVarNameLen, maxVarNameLen, k,
			maxTypeNameLen, maxTypeNameLen, vi.typeName,
			vi.desc)
		sep = "\n"
	}
	return kvl
}

// gPrint prints the text with the appropriate indent and the Gosh comment
func (g *Gosh) gPrint(s, tag string) {
	if s == "" {
		fmt.Fprintln(g.w)
		return
	}

	fmt.Fprintln(g.w, g.indentStr()+s+g.comment(tag))
}

// gPrintErr prints a line that reports an error with the appropriate indent
// and the Gosh comment
func (g *Gosh) gPrintErr(s, tag string) {
	fmt.Fprintln(g.w,
		g.indentStr()+"fmt.Fprintf(os.Stderr, "+s+")"+g.comment(tag))
}

// print prints the text with the appropriate indent and no comment. This
// should be used for user-supplied code
func (g *Gosh) print(s string) {
	fmt.Fprintln(g.w, g.indentStr()+s)
}
