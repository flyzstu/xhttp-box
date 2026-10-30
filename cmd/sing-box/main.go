//go:build !generate

package main

import (
	_ "time/tzdata"

	"github.com/sagernet/sing-box/log"
)

func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}
