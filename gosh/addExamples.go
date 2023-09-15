package main

import (
	"fmt"

	"github.com/nickwells/param.mod/v6/param"
)

// addExamples adds some examples of how gosh might be used to the standard
// help message
func addExamples(ps *param.PSet) error {
	ps.AddExample(`gosh -pln '"Hello, World!"'`, `This prints Hello, World!`)

	ps.AddExample("gosh -pln 'math.Pi'", `This prints the value of Pi`)

	ps.AddExample("gosh -pln '17*12.5'",
		`This prints the results of a simple calculation`)

	ps.AddExample(
		"gosh -n -b 'count := 0' -e 'count++' -a-pln 'count'",
		"This reads from the standard input and prints"+
			" the number of lines read"+
			"\n\n"+
			"-n sets up the loop reading from standard input"+
			"\n\n"+
			"-b 'count := 0' declares and initialises the counter"+
			" before the loop"+
			"\n\n"+
			"-e 'count++' increments the counter inside the loop"+
			"\n\n"+
			"-a-pln 'count' prints the counter using fmt.Println"+
			" after the loop.")

	ps.AddExample("gosh -n -b-p '\"Radius: \"'"+
		" -e 'r, err := strconv.ParseFloat(_l.Text(), 64)'"+
		" -e-s iferr"+
		" -pf '\"Area: %9.2f\\n\", r*r*math.Pi'"+
		" -p '\"Radius: \"'",
		"This repeatedly prompts the user for a Radius and prints"+
			" the Area of the corresponding circle"+
			"\n\n"+
			"-n sets up the loop reading from standard input"+
			"\n\n"+
			"-b-p '\"Radius: \"' prints the first prompt"+
			" before the loop."+
			"\n\n"+
			"-e 'r, err := strconv.ParseFloat(_l.Text(), 64)' sets"+
			" the radius from the text read from standard input,"+
			" ignoring errors."+
			"\n\n"+
			"-e-s iferr checks the error using the 'iferr' snippet"+
			"\n\n"+
			"-pf '\"Area: %9.2f\\n\", r*r*math.Pi' calculates and"+
			" prints the area using fmt.Printf."+
			"\n\n"+
			"-p '\"Radius: \"' prints the next prompt.")

	ps.AddExample(
		`gosh -i -w-pln `+
			`'strings.ReplaceAll(string(_l.Text()), "mod/pkg", "mod/v2/pkg")'`+
			` -- abc.go xyz.go `,
		"This changes each line in the two files abc.go and xyz.go"+
			" replacing any reference to mod/pkg with mod/v2/pkg. You"+
			" might find this useful when you are upgrading a Go module"+
			" which has changed its major version number."+
			"\n\n"+
			"The files will be changed and the original contents will"+
			" be left behind in files called abc.go.orig and xyz.go.orig."+
			"\n\n"+
			"-i sets up the edit-in-place behaviour"+
			"\n\n"+
			"-w-pln writes to the new, edited copy of the file")

	ps.AddExample(
		`gosh -i`+
			` -e 'if _fl == 1 {'`+
			` -w-pln '"// Edited by Gosh!"'`+
			` -w-pln ''`+
			` -e '}'`+
			` -w-pln '_l.Text()'`+
			` -- abc.go xyz.go `,
		"This edits the two files abc.go and xyz.go adding a comment at"+
			" the top of each file. It finds the top of the file by"+
			" checking the built-in variable _fl which gives the line"+
			" number in the current file"+
			"\n\n"+
			"The files will be changed and the original contents will"+
			" be left behind in files called abc.go.orig and xyz.go.orig."+
			"\n\n"+
			"-i sets up the edit-in-place behaviour"+
			"\n\n"+
			"-w-pln writes to the new, edited copy of the file")

	ps.AddExample(`gosh -http-handler 'http.FileServer(http.Dir("/tmp/xxx"))'`,
		"This runs a web server that serves files from /tmp/xxx.")

	ps.AddExample(`gosh -web-p '"Gosh!"'`,
		"This runs a web server (listening on port "+
			fmt.Sprint(dfltHTTPPort)+") that returns 'Gosh!' for every"+
			" request.")

	ps.AddExample(`gosh -n -e 'if l := len(_l.Text()); l > 80 { '`+
		` -pf '"%3d: %s\n", l, _l.Text()' -e '}'`,
		"This will read from standard input and print out each line that"+
			" is longer than 80 characters.")

	ps.AddExample(`gosh -snippet-list`,
		"This will list all the available snippets.")

	ps.AddExample(`gosh`+
		` -snippet-list`+
		` -snippet-list-short`+
		` -snippet-list-part text`+
		` -snippet-list-constraint iferr`,
		"This will list just the text of the iferr snippet.")

	return nil
}
