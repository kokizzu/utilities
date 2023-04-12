package main

import (
	"errors"
	"testing"

	"github.com/nickwells/testhelper.mod/v2/testhelper"
)

func TestPopulateFilesToRead(t *testing.T) {
	type tcs struct {
		testhelper.ID
		files   []string
		g       *Gosh
		expGosh *Gosh
	}
	var testCases []tcs

	{
		var g *Gosh
		var eg *Gosh
		noRemainder := []string{}

		g = mkTestGosh(func(g *Gosh) { g.runInReadLoop = true })
		eg = mkTestGosh(func(g *Gosh) { g.runInReadLoop = true })

		testCases = append(testCases, tcs{
			ID:      testhelper.MkID("no remainder, run-in-readloop"),
			files:   noRemainder,
			g:       g,
			expGosh: eg,
		})
	}

	{
		var g *Gosh
		var eg *Gosh
		remainder := []string{testDataFile1}

		g = mkTestGosh(func(g *Gosh) {
			g.runInReadLoop = true
			g.inPlaceEdit = true
		})
		eg = mkTestGosh(func(g *Gosh) {
			g.runInReadLoop = true
			g.inPlaceEdit = true
			g.filesToRead = true
			g.args = remainder
		})

		testCases = append(testCases, tcs{
			ID:      testhelper.MkID("one file, run-in-readloop"),
			files:   remainder,
			g:       g,
			expGosh: eg,
		})
	}

	{
		var g *Gosh
		var eg *Gosh
		remainder := []string{testDataFile1, testDataFile1}

		g = mkTestGosh(func(g *Gosh) {
			g.runInReadLoop = true
		})
		eg = mkTestGosh(func(g *Gosh) {
			g.runInReadLoop = true
			g.filesToRead = true
			g.args = []string{testDataFile1}
			g.addError("duplicate filename",
				errors.New("filename \"testdata/file1\""+
					" has been given more than once,"+
					" first at 0 and again at 1"))
		})

		testCases = append(testCases, tcs{
			ID:      testhelper.MkID("two files, duplicates, run-in-readloop"),
			files:   remainder,
			g:       g,
			expGosh: eg,
		})
	}

	{
		var g *Gosh
		var eg *Gosh
		remainder := []string{testHasOrigFile}

		g = mkTestGosh(func(g *Gosh) {
			g.runInReadLoop = true
			g.inPlaceEdit = true
		})
		eg = mkTestGosh(func(g *Gosh) {
			g.runInReadLoop = true
			g.inPlaceEdit = true
			g.addError("original file check",
				errors.New("path: \"testdata/hasOrigFile.orig\""+
					" shouldn't exist but does"))
		})

		testCases = append(testCases, tcs{
			ID:      testhelper.MkID("one file with .orig, in-place-edit"),
			files:   remainder,
			g:       g,
			expGosh: eg,
		})
	}

	for _, tc := range testCases {
		tc.g.populateFilesToRead(tc.files)
		if err := testhelper.DiffVals(*tc.g, *tc.expGosh); err != nil {
			t.Log(tc.IDStr())
			t.Errorf("\t: Failed: %s\n", err)
		}
	}
}
