package main

import "os"

func main() {
	os.Exit(int(Run(os.Args[1:], os.Stdout, os.Stderr)))
}
