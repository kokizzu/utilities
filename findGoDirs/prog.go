package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/nickwells/check.mod/v2/check"
	"github.com/nickwells/dirsearch.mod/v2/dirsearch"
	"github.com/nickwells/gogen.mod/gogen"
	"github.com/nickwells/location.mod/location"
	"github.com/nickwells/verbose.mod/verbose"
)

type dirToContentMap map[string]contentMap

// prog holds the parameters and current status of the program
type prog struct {
	baseDirs       []string
	skipDirs       []string
	pkgNames       []string
	filesWanted    []string
	filesMissing   []string
	fileREsWanted  []*regexp.Regexp
	fileREsMissing []*regexp.Regexp
	contentChecks  []ContentCheck
	dirContent     dirToContentMap

	noAction      bool
	showCheckName bool

	doAction map[string]bool

	actionFuncs map[string]actionFunc

	generateArgs []string
	installArgs  []string
	buildArgs    []string
	testArgs     []string

	dbgStack *verbose.Stack
}

// newProg returns a properly initialised prog structure
func newProg() *prog {
	return &prog{
		dirContent:  make(dirToContentMap),
		doAction:    make(map[string]bool),
		actionFuncs: makeActionMap(),

		dbgStack: &verbose.Stack{},
	}
}

// run is the starting point for the program, it should be called from main()
// after the command-line parameters have been parsed. Use the setExitStatus
// method to record the exit status and then main can exit with that status.
func (prog *prog) run() {
	defer prog.dbgStack.Start("run", os.Args[0])()

	sortedDirs := prog.findMatchingDirs()
	for _, d := range sortedDirs {
		prog.onMatchDo(d)
	}
}

// findMatchingDirs finds directories in any of the baseDirs matching the
// given criteria. Note that this just finds directories, excluding those:
//
// - called testdata
// - starting with a dot
// - starting with an underscore
//
// It does not perform any of the other tests, on package names, file
// presence etc.
func (prog *prog) findMatchingDirs() []string {
	defer prog.dbgStack.Start("findMatchingDirs",
		"Find dirs matching criteria")()

	var dirs []string

	dirChecks := []check.FileInfo{
		check.FileInfoName(check.Not(
			check.ValEQ("testdata"),
			"Ignore any directory called testdata")),
		check.FileInfoName(check.Not(
			check.StringHasPrefix[string]("_"),
			"Ignore directories with name starting with '_'")),
		check.FileInfoName(
			check.Or(
				check.Not(
					check.StringHasPrefix[string]("."),
					"Ignore hidden directories (including .git)"),
				check.ValEQ("."),
				check.ValEQ(".."),
			)),
	}

	for _, skipDir := range prog.skipDirs {
		dirChecks = append(dirChecks, check.FileInfoName(check.Not(
			check.ValEQ(skipDir),
			"Ignore any directory called "+skipDir)))
	}

	fileChecks := []check.FileInfo{check.FileInfoIsDir}
	fileChecks = append(fileChecks, dirChecks...)

	if len(prog.baseDirs) == 0 {
		prog.baseDirs = []string{"."}
	}

	for _, dir := range prog.baseDirs {
		matches, errs := dirsearch.FindRecursePrune(dir, -1,
			dirChecks,
			fileChecks...)
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "Error: %q : %v\n", dir, err)
		}

		for d := range matches {
			dirs = append(dirs, d)
		}
	}

	sort.Strings(dirs)

	return slices.Compact(dirs)
}

// onMatchDo performs the actions if the directory is a go package directory
// meeting the criteria
func (prog *prog) onMatchDo(dir string) {
	defer prog.dbgStack.Start("onMatchDo", "Act on matching dir: "+dir)()

	intro := prog.dbgStack.Tag()

	undo, err := cd(dir)
	if err != nil {
		verbose.Println(intro, " Skipping: couldn't chdir")
		return
	}
	defer undo()

	pkg, err := gogen.GetPackage()
	if err != nil { // it's not a package directory
		verbose.Println(intro, " Skipping: Not a package directory")
		return
	}

	if !prog.pkgMatches(pkg) {
		verbose.Println(intro, " Skipping: Wrong package")
		return
	}

	if !prog.fileChecksOK() {
		verbose.Println(intro, " Skipping: file criteria not met")
		return
	}

	if !prog.hasRequiredContent(dir) {
		delete(prog.dirContent, dir)
		verbose.Println(intro, " Skipping: missing required content")

		return
	}

	prog.doAllActions(dir)
}

// cd will change directory to the given directory name and return a function
// to be called to get back to the original directory
func cd(dir string) (func(), error) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot get the current directory:", err)
		return nil, err
	}

	err = os.Chdir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot chdir to %q: %v\n", dir, err)
		return nil, err
	}

	return func() {
		os.Chdir(cwd) //nolint: errcheck
	}, nil
}

// pkgMatches will compare the package name against the list of target
// packages, if any, and return true only if any of them match. If there are
// no names to match then any name will match.
func (prog *prog) pkgMatches(pkg string) bool {
	if len(prog.pkgNames) == 0 { // any name matches
		return true
	}

	if slices.Contains(prog.pkgNames, pkg) {
		return true
	}

	return false // no name matches
}

// fileChecksOK checks the current directory for the file requirements. It
// returns false if the directory should be skipped, true otherwise.
func (prog *prog) fileChecksOK() bool {
	defer prog.dbgStack.Start("fileChecksOK", "checking files")()

	intro := prog.dbgStack.Tag()

	if ok, missing := hasEntries(prog.filesWanted); !ok {
		verbose.Println(intro, " missing files: ", missing)
		return false
	}

	if len(prog.filesMissing) > 0 {
		if ok, _ := hasEntries(prog.filesMissing); ok {
			missingFiles := strings.Join(prog.filesMissing, ", ")
			verbose.Println(intro, " not missing any files: ", missingFiles)

			return false
		}
	}

	if ok, missing := hasEntriesLike(prog.fileREsWanted); !ok {
		verbose.Println(intro, " missing files like: ", missing)
		return false
	}

	if len(prog.fileREsMissing) > 0 {
		if ok, _ := hasEntriesLike(prog.fileREsMissing); ok {
			patterns := []string{}
			for _, re := range prog.fileREsMissing {
				patterns = append(patterns, re.String())
			}

			missingPatterns := strings.Join(patterns, ", ")
			verbose.Println(intro, " not missing any files like: ",
				missingPatterns)

			return false
		}
	}

	verbose.Println(intro, " all file criteria are met")

	return true
}

// hasEntries will check to see if any of the listed directory entries exists
// in the current directory and return false if any of them are missing. It
// will only return true if all the entries are found in the directory
func hasEntries(entries []string) (bool, string) {
	if len(entries) == 0 {
		return true, ""
	}

	dirEntries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read the directory:", err)
		return false, "cannot read directory"
	}

	for _, entryName := range entries {
		if !entryFound(entryName, dirEntries) {
			return false, entryName
		}
	}

	return true, ""
}

// hasEntriesLike will check to see if there is at least one entry in the
// current directory for each of the supplied regexp's. It will return false
// if any of them are missing. It will only return true if there is a match
// for all the supplied regexp's.
func hasEntriesLike(entryREs []*regexp.Regexp) (bool, string) {
	if len(entryREs) == 0 {
		return true, ""
	}

	dirEntries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read the directory:", err)
		return false, "cannot read the directory"
	}

	for _, re := range entryREs {
		if !entryMatches(re, dirEntries) {
			return false, re.String()
		}
	}

	return true, ""
}

// entryFound will return true if the name is in the list of directory
// entries
func entryFound(name string, entries []fs.DirEntry) bool {
	for _, f := range entries {
		if f.Name() == name {
			return true
		}
	}

	return false
}

// entryMatches will return true if some name in the list of directory
// entries matches the regexp.
func entryMatches(re *regexp.Regexp, entries []fs.DirEntry) bool {
	for _, f := range entries {
		if re.MatchString(f.Name()) {
			return true
		}
	}

	return false
}

// hasRequiredContent will check to see if any of the files in the current
// directory has the required content and return false if any of the required
// content is not in any file. It will only return true if all the required
// content is present in at least one of the files in the directory. In any
// case the map of content discovered for the given directory will have been
// populated.
func (prog *prog) hasRequiredContent(dir string) bool {
	prog.dirContent[dir] = contentMap{}

	if len(prog.contentChecks) == 0 {
		return true
	}

	dirEntries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read the directory:", err)
		return false
	}

	for _, entry := range dirEntries {
		if !entry.Type().IsRegular() {
			continue
		}

		err := prog.checkContent(dir, entry.Name())
		if err != nil {
			break
		}
	}

	return len(prog.dirContent[dir]) == len(prog.contentChecks)
}

// checkContent opens the file and finds any content matching the checks,
// writing it into the contentMap. It returns a non-nil error if the file
// cannot be opened.
func (prog *prog) checkContent(dir, fname string) error {
	statusChecks := []StatusCheck{}

	for _, c := range prog.contentChecks {
		if c.FileNameOK(fname) {
			statusChecks = append(statusChecks, StatusCheck{chk: &c})
		}
	}

	if len(statusChecks) == 0 {
		return nil
	}

	pathname := filepath.Join(dir, fname) // for error reporting

	// Use fname not pathname - the open is relative to the current directory
	// so if the directory name is a relative path (containing '..') then
	// the pathname will not necessarily be available when the fname is.
	// This is because the process's working directory will have changed as
	// the process walks the tree of directories searching for matches.
	f, err := os.Open(fname) //nolint:gosec
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't open %q: %v\n", pathname, err)
		return err
	}

	defer f.Close()

	loc := location.New(pathname)
	s := bufio.NewScanner(f)

	for s.Scan() {
		loc.Incr()

		allChecksComplete := true

		for _, sc := range statusChecks {
			if sc.CheckLine(s.Text()) {
				locCopy := *loc
				locCopy.SetContent(s.Text())
				prog.dirContent[dir][sc.chk.name] = append(
					prog.dirContent[dir][sc.chk.name], locCopy)
			}

			if !sc.stopped {
				allChecksComplete = false
			}
		}

		if allChecksComplete {
			break
		}
	}

	return nil
}
