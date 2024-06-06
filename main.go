/*
The core server program is a simple reverse proxy that handles https and issues
certificates on demand using let's encrypt.

Its job is very simple: to provide a coordination mechanism between multiple web
servers running on the same machine so they don't need to compete for listening
on ports 80 and 443

Instead, each web server listens on its own port, but can still accept incoming
requests from the outside world if they are sent to the domain it wants to
handle.

Each web server needs to simply send a command to the core server telling it
which domain to redirect to which port.

There's no json or http API. Instead, you send a text message over UDP.

# The UDP port is 40608

The message needs to look like this:

	"add example.com 5665"

The message is a string with no new line.

This will cause the server to associate the given domain with the given port.
Any incoming request to the server that sets the Host header to the requested
domain will be redirected (reverse-proxied) to port 5665 any incoming https
request that sets the Host header to `example.com` to port 5665

If you want to redirect multiple domains to the same port, just send multiple
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

If you build it with the `-tags release`, it will run in release mode, where it
will server https, using let's encrypt to issue certificates. It will
auto-redirect all http requests to https. It will detach itself from the shell.
ّIt will write log files to `logs/core.log` (relative to the current directory)
and use a log rotation scheme configured to not consume more than 200 MB of disk
space.

When running on Linux, instead of running it with sudo, give it permission to
listen on port 80 and 443 using capabilities:

	sudo setcap CAP_NET_BIND_SERVICE=+eip ./core_server

The core_server is idempotent: it's safe to run multiple times. If another
core_server process is already running, it will send it a signal to shutdown, so
the new process can take over the special UDP port.

It will also remember which domains it was redirecting to which ports.

The code is just a few hundred lines, and should be quite easy to read.
*/
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
