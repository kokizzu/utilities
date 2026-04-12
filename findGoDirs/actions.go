package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/nickwells/gogen.mod/gogen"
	"github.com/nickwells/verbose.mod/verbose"
)

const (
	printAct    = "print"
	buildAct    = "build"
	installAct  = "install"
	testAct     = "test"
	generateAct = "generate"
	contentAct  = "content"
	filenameAct = "filename"
)

type actionFunc func(*prog, string)

var actionsInOrder = []string{
	printAct, contentAct, filenameAct,
	generateAct, testAct, buildAct, installAct,
}

// doAllActions performs all the actions on the named directory in
// order. Note that the program should already have chdir'ed into this
// directory
func (prog *prog) doAllActions(dir string) {
	defer prog.dbgStack.Start("doAllActions", "acting")()

	intro := prog.dbgStack.Tag()

	for _, a := range actionsInOrder {
		if prog.doAction[a] {
			verbose.Println(intro, " Doing: "+a)
			prog.actionFuncs[a](prog, dir)
		}
	}
}

// makeActionMap returns a map of strings to action functions
func makeActionMap() map[string]actionFunc {
	return map[string]actionFunc{
		printAct:    doPrint,
		buildAct:    doBuild,
		installAct:  doInstall,
		testAct:     doTest,
		generateAct: doGenerate,
		contentAct:  doContent,
		filenameAct: doFilenames,
	}
}

// reportNoAction reports the action being skipped in a common format
func reportNoAction(actionName, dirName string) {
	fmt.Printf("%-20.20s : %s (no-action: skipping)\n", actionName, dirName)
}

// doPrint will print the name
func doPrint(prog *prog, dirName string) {
	if prog.noAction {
		reportNoAction("print", dirName)
		return
	}

	fmt.Println(dirName)
}

// doContent will show the lines in the files in the directory that match
// the content checks
func doContent(prog *prog, dirName string) {
	defer prog.dbgStack.Start("doContent",
		"Print matching content in : "+dirName)()

	if prog.noAction {
		reportNoAction("content", dirName)
		return
	}

	keys := slices.Sorted(maps.Keys(prog.dirContent[dirName]))

	maxKeyLen := 0

	if prog.showCheckName {
		for _, k := range keys {
			maxKeyLen = max(len(k), maxKeyLen)
		}
	}

	for _, k := range keys {
		prevSource := ""

		for _, match := range prog.dirContent[dirName][k] {
			source := match.Source()
			if prog.showCheckName {
				source = fmt.Sprintf("%*.*s: %s",
					maxKeyLen, maxKeyLen, k, source)
			}

			if source != prevSource {
				fmt.Printf("%s:", source)
			} else {
				fmt.Printf("%s:", strings.Repeat(" ", len(prevSource)))
			}

			content, _ := match.Content()
			fmt.Printf("%d:%s\n", match.Idx(), content)

			prevSource = source
		}
	}
}

// doFilenames will show the names of the files in the directories that match
// the content checks
func doFilenames(prog *prog, dirName string) {
	defer prog.dbgStack.Start("doFilenames",
		"Print files with matching content in : "+dirName)()

	if prog.noAction {
		reportNoAction("filenames", dirName)
		return
	}

	keys := slices.Sorted(maps.Keys(prog.dirContent[dirName]))

	for _, k := range keys {
		for _, match := range prog.dirContent[dirName][k] {
			fmt.Println(match.Source())
		}
	}
}

// doBuild will run go build
func doBuild(prog *prog, dirName string) {
	prog.doGoCommand(dirName, "build", prog.buildArgs)
}

// doTest will run go test
func doTest(prog *prog, dirName string) {
	prog.doGoCommand(dirName, "test", prog.testArgs)
}

// doInstall will run go install
func doInstall(prog *prog, dirName string) {
	prog.doGoCommand(dirName, "install", prog.installArgs)
}

// doGenerate will run go generate
func doGenerate(prog *prog, dirName string) {
	prog.doGoCommand(dirName, "generate", prog.generateArgs)
}

// doGoCommand will run the Go subcommand with the passed args
func (prog *prog) doGoCommand(dirName, command string, cmdArgs []string) {
	defer prog.dbgStack.Start("doGoCommand", "In : "+dirName)()

	intro := prog.dbgStack.Tag()

	if prog.noAction {
		reportNoAction("go "+command, dirName)
		return
	}

	args := []string{command}
	args = append(args, cmdArgs...)

	verbose.Println(intro, "go "+strings.Join(args, " "))
	gogen.ExecGoCmd(gogen.ShowCmdIO, args...)
}
