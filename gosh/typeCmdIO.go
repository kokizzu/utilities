package  main

// This code was generated by mkfunccontrolparamtype
// with parameters set at:
//	command line:4: -d This determines whether or not a go subcommand should be run with its output displayed
//	command line:2: -t CmdIO
//	command line:6: -v None
//	command line:8: -v Show
//
// *** DO NOT EDIT ***

// CmdIO This determines whether or not a go subcommand should be run with its output displayed
type CmdIO int

const (
	TCmdIOMinVal CmdIO = iota
	TCmdIONone
	TCmdIOShow
	TCmdIOMaxVal
)

// IsValid is a method on the CmdIO type that can be used
// to check a received parameter and return an error
// or panic if an illegal parameter value is passed
func (v CmdIO)IsValid() bool {
	if v <= TCmdIOMinVal {
		return false
	}
	if v >= TCmdIOMaxVal {
		return false
	}
	return true
}