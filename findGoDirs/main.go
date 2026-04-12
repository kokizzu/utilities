package main

// Created: Thu Jun 11 12:43:33 2020

func main() {
	prog := newProg()
	ps := makeParamSet(prog)

	ps.Parse()

	prog.run()
}
