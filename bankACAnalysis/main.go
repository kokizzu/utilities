// bankACAnalysis
package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nickwells/col.mod/v2/col"
	"github.com/nickwells/col.mod/v2/col/colfmt"
	"github.com/nickwells/location.mod/location"
	"github.com/nickwells/param.mod/v3/param"
	"github.com/nickwells/param.mod/v3/param/paramset"
	"github.com/nickwells/param.mod/v3/param/psetter"
)

// Created: Sun May 12 16:39:24 2019

// Xactn represents a single transaction
type Xactn struct {
	lineNum   int
	date      time.Time
	xaType    string
	desc      string
	debitAmt  float64
	creditAmt float64
	balance   float64
}

// Summary represents a summary of the account transactions
type Summary struct {
	name       string
	count      int
	firstDate  time.Time
	lastDate   time.Time
	debitAmt   float64
	creditAmt  float64
	parent     *Summary
	depth      int
	components map[string]*Summary
}

const (
	catAll     = "all"
	catUnknown = "unknown"
	catCash    = "cash"
	catCheque  = "cheque"
)

const xactnMapDesc = "map of transaction types"

const tabWidth = 4

// Edit represents a substitution to be made to a transaction description
type Edit struct {
	search      string
	searchRE    *regexp.Regexp
	replacement string
}

type Summaries struct {
	parentOf     map[string]string
	summaries    map[string]*Summary
	edits        []Edit
	maxDepth     int
	maxNameWidth int
}

type reportStyle int

const (
	showLeafEntries reportStyle = iota
	summaryReport
)

// openFileOrDie will try to open the given file and will return the open
// file if successful and will print an error message and exit of not.
func openFileOrDie(fileName, desc string) *os.File {
	f, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("Couldn't open the %s file: %s", desc, err)
		os.Exit(1)
	}
	return f
}

// populateParents constructs the parent tree of transactions from the
// transaction map file
func (s *Summaries) populateParents() {
	s.parentOf[catAll] = catAll
	err := s.addParent(catAll, catUnknown)
	if err != nil {
		fmt.Printf("Cannot initialise the %s: %s\n", xactnMapDesc, err)
		os.Exit(1)
	}
	err = s.addParent(catAll, catCash)
	if err != nil {
		fmt.Printf("Cannot initialise the %s: %s\n", xactnMapDesc, err)
		os.Exit(1)
	}
	err = s.addParent(catAll, catCheque)
	if err != nil {
		fmt.Printf("Cannot initialise the %s: %s\n", xactnMapDesc, err)
		os.Exit(1)
	}

	mf := openFileOrDie(xactMapFileName, xactnMapDesc)
	defer mf.Close()

	mScanner := bufio.NewScanner(mf)
	lineNum := 0

	for mScanner.Scan() {
		lineNum++
		line := mScanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		err = s.addParent(parts[0], parts[1])
		if err != nil {
			fmt.Printf("%s:%d: Bad entry in the %s: %s",
				xactMapFileName, lineNum, xactnMapDesc, err)
		}
	}
}

// populateEdits constructs the slice of editing rules to be performed on
// transaction descriptions
func (s *Summaries) populateEdits() {
	ef := openFileOrDie(editFileName, "transaction edits")
	defer ef.Close()

	eScanner := bufio.NewScanner(ef)
	lineNum := 0
	prevType := ""
	var searchRE *regexp.Regexp
	var searchStr string
	var errFound bool
	var err error
	const errIntro = "Bad transaction edits entry"

	for eScanner.Scan() {
		lineNum++
		line := eScanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("%s:%d: %s: Missing '=' : %s\n",
				editFileName, lineNum, errIntro, line)
			errFound = true
			continue
		}
		entryType := parts[0]
		switch entryType {
		case "search":
			if prevType == "search" {
				fmt.Printf(
					"%s:%d: %s: Replace entry missing for previous search\n",
					editFileName, lineNum, errIntro)
			}
			errFound = false
			searchStr = parts[1]
			searchRE, err = regexp.Compile(searchStr)
			if err != nil {
				fmt.Printf("%s:%d: %s: Couldn't compile the regexp: %s\n",
					editFileName, lineNum, errIntro, err)
				errFound = true
			}
		case "replace":
			if !errFound {
				s.edits = append(s.edits, Edit{
					search:      searchStr,
					searchRE:    searchRE,
					replacement: parts[1],
				})
			}
		default:
			fmt.Printf("%s:%d: %s: Bad type: %s\n",
				editFileName, lineNum, errIntro, entryType)
			errFound = true
		}
		prevType = entryType
	}
}

// initSummaries returns an initialised Summaries structure
func initSummaries() *Summaries {
	s := Summaries{
		parentOf:  make(map[string]string),
		summaries: make(map[string]*Summary),
	}

	s.summaries[catAll] = &Summary{
		name:       catAll,
		components: make(map[string]*Summary),
	}
	s.populateParents()

	s.populateEdits()

	return &s
}

// addParent adds the parent/child relationship so that a given summary can
// find its parent. It is an error if the parent does not already exist.
func (s *Summaries) addParent(parent, child string) error {
	if _, ok := s.parentOf[parent]; !ok {
		return fmt.Errorf("the parent %q of child %q does not exist",
			parent, child)
	}

	if oldParent, ok := s.parentOf[child]; ok {
		if oldParent != parent {
			return fmt.Errorf("child %q already has a parent: %q != %q",
				child, parent, oldParent)
		}
		return nil
	}

	pSum := s.summaries[parent]
	cSum := &Summary{
		name:       child,
		parent:     pSum,
		depth:      pSum.depth + 1,
		components: make(map[string]*Summary),
	}
	s.summaries[child] = cSum
	if cSum.depth > s.maxDepth {
		s.maxDepth = cSum.depth
	}
	if len(cSum.name) > s.maxNameWidth {
		s.maxNameWidth = len(cSum.name)
	}

	pSum.components[child] = cSum
	s.parentOf[child] = parent
	return nil
}

// summarise will summarise the transaction working its way up to the top of
// the tree of Summary records
func (s *Summaries) summarise(xa Xactn) {
	summ := s.summaries[xa.desc]
	summ.add(xa)
}

// add will add the values to the summary record and move on to the parent
// (if there is one)
func (s *Summary) add(xa Xactn) {
	if s.count == 0 {
		s.firstDate = xa.date
		s.lastDate = xa.date
	} else {
		if xa.date.After(s.lastDate) {
			s.lastDate = xa.date
		} else if s.firstDate.After(xa.date) {
			s.firstDate = xa.date
		}
	}
	s.count++
	s.debitAmt += xa.debitAmt
	s.creditAmt += xa.creditAmt
	if s.parent != nil {
		s.parent.add(xa)
	}
}

// the name of the file containing the transactions
var acFileName string

// the name of the file containing the replacements to make to transaction
// names
var editFileName string

// the name of the file containing the mappings between transaction names and
// categories
var xactMapFileName string

// don't suppress printing of summary records for which there are no
// transactions
var showZeros bool

var style = showLeafEntries

var minimalAmount float64

func main() {
	ps := paramset.NewOrDie(addParams,
		param.SetProgramDescription(`analyse the bank account`))
	ps.Parse()

	f := openFileOrDie(acFileName, "bank account")
	defer f.Close()
	r := csv.NewReader(f)

	summaries := initSummaries()

	lineNum := 0
	for {
		parts, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		lineNum++
		if lineNum == 1 {
			continue // ignore the first line of headings
		}
		xa, err := mkXactn(lineNum, parts)
		if err != nil {
			fmt.Println(err)
			continue
		}
		summaries.createNewMapEntries(acFileName, lineNum, xa)
		summaries.summarise(xa)
	}

	summaries.report(style)
}

// createNewMapEntries will create new parent/child map entries for the
// transaction if it is not already known or if it is a cheque or cashpoint
// withdrawal
func (s *Summaries) createNewMapEntries(fileName string, lineNum int, xa Xactn) {
	if xa.xaType == "CHQ" {
		err := s.addParent(catCheque, xa.desc)
		if err != nil {
			fmt.Printf(
				"%s:%d: Can't add the cheque to the %s: %s\n",
				fileName, lineNum, xactnMapDesc, err)
		}
	} else if xa.xaType == "CPT" {
		err := s.addParent(catCash, xa.desc)
		if err != nil {
			fmt.Printf(
				"%s:%d: Can't add the cashpoint withdrawal to the %s: %s\n",
				fileName, lineNum, xactnMapDesc, err)
		}
	} else {
		if _, ok := s.parentOf[xa.desc]; ok {
			return
		}

		xa.desc = s.normalise(xa.desc)
		if _, ok := s.parentOf[xa.desc]; ok {
			return
		}

		err := s.addParent(catUnknown, xa.desc)
		if err != nil {
			fmt.Printf(
				"%s:%d: Can't add the unknown entry to the %s: %s\n",
				fileName, lineNum, xactnMapDesc, err)
		}
	}
}

// normalise converts the string into a 'normal' form - this involves editing
// it to replace multiple alternative spellings into a single variant. It
// returns after the first edit which changes the string
func (s *Summaries) normalise(str string) string {
	for _, ed := range s.edits {
		newS := ed.searchRE.ReplaceAllLiteralString(str, ed.replacement)
		if newS != str {
			return newS
		}
	}
	return str
}

//  report will report the summaries
func (s *Summaries) report(style reportStyle) {
	summ := s.summaries[catAll]

	h, err := col.NewHeader()
	if err != nil {
		fmt.Println("Error found while constructing the header:", err)
		return
	}

	floatCol := colfmt.Float{
		W:    10,
		Prec: 2,
		Zeroes: &colfmt.FloatZeroHandler{
			Handle:  true,
			Replace: "",
		},
	}
	pctCol := colfmt.Percent{
		W: 5,
		Zeroes: &colfmt.FloatZeroHandler{
			Handle:  true,
			Replace: "",
		},
	}

	rpt, err := col.NewReport(h, os.Stdout,
		col.New(colfmt.String{W: tabWidth*s.maxDepth + s.maxNameWidth},
			"Transaction Type"),
		col.New(colfmt.Int{W: 5}, "Count"),
		col.New(&colfmt.Time{Format: "2006-Jan-02"},
			"Date of", "First", "Transaction"),
		col.New(&colfmt.Time{Format: "2006-Jan-02"},
			"Date of", "Last", "Transaction"),
		col.New(&floatCol, "Debit", "Amount"),
		col.New(&pctCol, "%age"),
		col.New(&floatCol, "Credit", "Amount"),
		col.New(&pctCol, "%age"),
		col.New(&floatCol, "Nett", "Amount"),
	)
	if err != nil {
		fmt.Println("Error found while constructing the report:", err)
		return
	}

	summ.report(rpt, summ.debitAmt, summ.creditAmt, 0, style)
}

func (s *Summary) report(rpt *col.Report, totDebit, totCredit float64, indent int, style reportStyle) {
	if style == summaryReport && len(s.components) == 0 {
		return
	}
	if !showZeros && s.count == 0 {
		return
	}
	if s.creditAmt+s.debitAmt < minimalAmount {
		return
	}

	err := rpt.PrintRow(
		strings.Repeat(" ", tabWidth*indent)+s.name,
		s.count,
		s.firstDate, s.lastDate,
		s.debitAmt, s.debitAmt/totDebit,
		s.creditAmt, s.creditAmt/totCredit,
		s.creditAmt-s.debitAmt)
	if err != nil {
		fmt.Println("Couldn't print the row:", err)
	}

	compList := []*Summary{}
	for _, c := range s.components {
		compList = append(compList, c)
	}
	sort.Slice(compList, func(i, j int) bool {
		return (compList[i].debitAmt + compList[i].creditAmt) >
			(compList[j].debitAmt + compList[j].creditAmt)
	})
	for _, c := range compList {
		c.report(rpt, totDebit, totCredit, indent+1, style)
	}
}

// addParams will add parameters to the passed ParamSet
func addParams(ps *param.PSet) error {
	ps.Add("ac-file", psetter.Pathname{Value: &acFileName},
		"the name of the file containing the bank account transactions",
		param.Attrs(param.MustBeSet))

	ps.Add("map-file", psetter.Pathname{Value: &xactMapFileName},
		"the name of the file containing the transaction name map",
		param.Attrs(param.MustBeSet))

	ps.Add("edit-file", psetter.Pathname{Value: &editFileName},
		"the name of the file containing the transaction name replacements",
		param.Attrs(param.MustBeSet))

	ps.Add("show-zeroes", psetter.Bool{Value: &showZeros},
		"don't suppress entries which have no transactions")

	ps.Add("summary", psetter.Nil{},
		"show a summary report with no leaf transactions",
		param.PostAction(func(_ location.L,
			_ *param.ByName,
			_ []string) error {
			style = summaryReport
			return nil
		}))

	ps.Add("minimal-amount", psetter.Float64{Value: &minimalAmount},
		"don't show summaries where the total transactions are less than this")

	return nil
}

// parseNum returns 0.0 if the string is empty, otherwise it will parse the
// number as a float
func parseNum(s, name string) (float64, error) {
	if s == "" {
		return 0.0, nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0, fmt.Errorf("Couldn't parse the %s: %s", name, err)
	}
	return n, nil
}

// mkXactn converts the slice of strings into an transaction record
func mkXactn(lineNum int, parts []string) (Xactn, error) {
	date, err := time.Parse("02/01/2006", parts[0])
	if err != nil {
		return Xactn{}, fmt.Errorf("Couldn't parse the date: %s", err)
	}

	da, err := parseNum(parts[5], "debit amount")
	if err != nil {
		return Xactn{}, err
	}

	ca, err := parseNum(parts[6], "debit amount")
	if err != nil {
		return Xactn{}, err
	}

	bal, err := parseNum(parts[7], "balance amount")
	if err != nil {
		return Xactn{}, err
	}

	return Xactn{
		lineNum:   lineNum,
		date:      date,
		xaType:    parts[1],
		desc:      parts[4],
		debitAmt:  da,
		creditAmt: ca,
		balance:   bal,
	}, nil
}