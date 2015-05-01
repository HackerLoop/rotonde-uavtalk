package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		panic(fmt.Sprintf("Usage: %s uavobject_directory", os.Args[0]))
	}

	loadUAVObjectDefinitions(os.Args[1])

	c := startUAVTalk()

	for range c {

	}

	return
}
