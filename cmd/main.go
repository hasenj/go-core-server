/*
The core server program is a simple reverse proxy that handles https and issues
certificates on demand using let's encrypt.

It's job is very simple: to provide a coordination mechanism between multiple
web servers running on the same machine so they don't need to compete for
listening to ports 80 and 443

Instead, each web server can specify its own port, and simply send a command to
the core server telling it which domain to redirect to which port.

To do that, simple send a UDP message to port 40608 that looks like this:

	"add example.com 5665"

The message is a string byte with no new line.

This will cause the server to reverse-proxy any incoming https request that sets
the Host header to `example.com` to port 5665

For this to be effective, you need your server program to listen to incoming
http connections on port 5665.

If you want to redirect multiple domains to the same port, simply send multiple
commands, each specifying a different domain.

If the domain was previously specified to redirect to a different port, it will
be overridden. That's right: there are no "security" features. It's assumed that
you trust the server programs that send these commands.

The only security feature is that it will reject any UDP message not coming from
127.0.0.1, so that no one running on a different machine can send a command to
core server directly.

If you build this program without any build tags, it will run in "local dev
mode". It will only handle https if it can detect that `mkcert` command is
installed. If it cannot detect it, it will only serve http requests.

If you build it with the `-tags release`, it will run in release mode.

In this mode, you can simply run it from the command line and disown it:

	./core_server & disown

It will write log files to `logs/core.log` and use a log rotation scheme
configured to not consume more than 200 MB of disk space.

When running on Linux, instead of running it with sudo, give it permission to
listen on port 80 and 443 using capabilities:

	sudo setcap CAP_NET_BIND_SERVICE=+eip ./core_server
*/
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
