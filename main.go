package main

import (
	"log"

	"github.com/vaikas/pombump/cmd/pombump"
)

func main() {
	if err := pombump.RootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}
