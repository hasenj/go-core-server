package main

import (
	"log"

	core "go.hasen.dev/core_server"
)

func main() {
	core.InitLogger()
	log.Println()
	log.Println("Starting Core Server")
	proxy := core.NewCoreServer()
	proxy.Start()
}
