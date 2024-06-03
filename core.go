package core_server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/generic"
)

// port used by proxy server to communicate with target servers
const LOCAL_UDP_PORT = 40608

const (
	ADD_TARGET    = "add"
	REMOVE_TARGET = "remove"
	SHUTDOWN      = "shutdown"
)

// some other process
type ForwardTarget struct {
	Domain string
	Port   int
}

func ParseForwardTarget(line string, target *ForwardTarget) error {
	_, err := fmt.Sscanf(line, "%s %d", &target.Domain, &target.Port)
	return err
}

func PrintForwardTarget(target *ForwardTarget) string {
	return fmt.Sprintf("%s %d", target.Domain, target.Port)
}

func ParseTargetList(data []byte, list *[]ForwardTarget) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var target ForwardTarget
		err := ParseForwardTarget(line, &target)
		if err != nil {
			log.Println(err)
		} else {
			generic.Append(list, target)
		}
	}
}

func WriteTargetList(list []ForwardTarget) string {
	var lines []string
	for _, target := range list {
		generic.Append(&lines, PrintForwardTarget(&target))
	}
	return strings.Join(lines, "\n")
}

type CoreServer struct {
	ConfigFile string
	Targets    []ForwardTarget
}

func NewCoreServer() *CoreServer {
	var s CoreServer
	log.Println("NewCoreServer")

	configRoot, err := os.UserConfigDir()
	if err != nil {
		log.Println(err)
	}
	if err == nil {
		configDir := filepath.Join(configRoot, "core-web-server")
		err := os.MkdirAll(configDir, 0777)
		if err != nil {
			log.Println(err)
		}

		s.ConfigFile = filepath.Join(configDir, "forward_targets.txt")
		log.Println("Using config file:", s.ConfigFile)

		data, _ := os.ReadFile(s.ConfigFile)
		if len(data) > 0 {
			ParseTargetList(data, &s.Targets)
			for _, t := range s.Targets {
				log.Printf("Serving http://%s\n", t.Domain)
			}
		}

		// TODO: watch for changes?
	}

	return &s
}

// ServeHTTP implements http.Handler
// it forwards a request to a server/handler based on the domain
func (s *CoreServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer log.Println(req.RemoteAddr, req.Host, req.Method, req.RequestURI)
	var host = req.Host
	for _, target := range s.Targets {
		if target.Domain == host {
			ForwardRequestLocally(w, req, target)
			return
		}
	}
	http.Error(w, "Domain Not Recognized", 404)
}

func ForwardRequestLocally(w http.ResponseWriter, req *http.Request, t ForwardTarget) {
	// var targetUrl, _ = url.Parse(fmt.Sprintf("http://localhost:%d/", t.Port))
	var targetUrl = url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", t.Port),
	}
	var rev = httputil.NewSingleHostReverseProxy(&targetUrl)
	rev.ServeHTTP(w, req)
}

// helper functions for app servers that don't swap themselves in
func AnnounceForwardTarget(domain string, port int) {
	log.Println("Announcing", domain, port)
	var t ForwardTarget
	t.Domain = domain
	t.Port = port

	writeCommand(ADD_TARGET, PrintForwardTarget(&t))

	var udpAddress = net.UDPAddr{Port: LOCAL_UDP_PORT}
	conn, err := net.DialUDP("udp", nil, &udpAddress)
	if err != nil {
		log.Println("Could not announce server", domain, err)
	}
	_, err = fmt.Fprintf(conn, ADD_TARGET+" "+PrintForwardTarget(&t))
	if err != nil {
		log.Println("AnnounceForwardTarget: write error", err)
	}

	conn.Close()
}

func writeCommand(args ...string) error {
	var udpAddress = net.UDPAddr{Port: LOCAL_UDP_PORT}
	conn, err := net.DialUDP("udp", nil, &udpAddress)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = fmt.Fprintf(conn, strings.Join(args, " "))
	return err
}

func splitCommand(line string) (string, string) {
	cmd, arg, _ := strings.Cut(line, " ")
	return cmd, arg
}

func HandleAddTargetRequest(s *CoreServer, content string) {
	var target ForwardTarget
	err := ParseForwardTarget(content, &target)
	if err != nil {
		log.Println("Invalid target:", err)
		return
	}
	if target.Domain == "" {
		log.Println("Attempting to add target without a domain; refusing")
		return
	}

	found := false
	for i := range s.Targets {
		if s.Targets[i].Domain == target.Domain {
			s.Targets[i] = target
			found = true
		}
	}
	if !found {
		generic.Append(&s.Targets, target)
	}

	log.Printf("Serving http://%s\n", target.Domain)

	if s.ConfigFile != "" {
		content := WriteTargetList(s.Targets)
		log.Println("Updating config file", s.ConfigFile)
		log.Println("Content:", content)
		os.WriteFile(s.ConfigFile, generic.UnsafeStringBytes(content), 0644)
	}
}

func shutdownPreviousInstance() {
	// try listening to the port first
	var port = LOCAL_UDP_PORT
	var udpAddress = net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", &udpAddress)
	if conn != nil {
		conn.Close()
	}

	// If listening failed, another instance is running. Shutdown it down!
	if err != nil {
		log.Println("Sending shutdown command")
		err := writeCommand(SHUTDOWN)
		if err != nil {
			log.Println("error sending shutdown command:", err)
		}
		// small delay to wait for the other instance to release all its ports, etc, to the OS
		time.Sleep(time.Millisecond * 10)
	}
}

func startUDPServer(s *CoreServer) {
	// this whoel thing runs inside a go routine
	var port = LOCAL_UDP_PORT
	var udpAddress = net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", &udpAddress)
	if err != nil {
		log.Println("Could not start UDP server:", err)
		return
	}

	log.Println("Started UDP server")

	for {
		var data = make([]byte, 1024)
		// this should block until someone sends us a udp message
		length, remoteAddr, err := conn.ReadFromUDP(data)
		if err != nil {
			log.Println("ReadFromUDP failed")
			continue
		}
		if !remoteAddr.IP.IsLoopback() {
			log.Println("Non-Lookback IP:", remoteAddr.IP.String())
			continue
		}

		data = data[:length]
		msg := generic.UnsafeString(data)

		log.Println("New UDP message:", msg)
		// handle the connection.
		//
		// NOTE: no goroutine; we should handle it very quickly and we don't
		// expect lots of concurrent connections infact we expect very long
		// times to pass without hardly any incoming connection
		cmd, arg := splitCommand(msg)
		switch cmd {
		case ADD_TARGET:
			HandleAddTargetRequest(s, arg)
		case SHUTDOWN:
			log.Println("Recieved shutdown command!")
			go os.Exit(0)
		}
	}
}

func httpToHttpsRedirector() {
	redirector := http.NewServeMux()
	redirector.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		var url = "https://" + req.Host + req.RequestURI
		http.Redirect(w, req, url, http.StatusTemporaryRedirect)
	})
	log.Println("Redirecting http to https")
	ln, err := net.Listen("tcp", ":http")
	if err != nil {
		log.Println(err)
	} else {
		http.Serve(ln, redirector)
	}
}
