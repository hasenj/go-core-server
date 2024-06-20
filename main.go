package main

import lib "go.hasen.dev/core_server/lib"

func main() {
	if ShouldDetach {
		lib.DetachFromParentProcess()
	}
	InitLogger()
	c := NewCoreServerWithConfigLoaded()
	StartCoreServer(c)
}
