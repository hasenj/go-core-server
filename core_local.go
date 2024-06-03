//go:build !release

package core_server

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

func cmdInstalled(c string) bool {
	_, err := exec.LookPath(c)
	return err == nil
}

func _keyFile(domain string) string {
	return "certs/" + domain + "-key.pem"
}

func _certFile(domain string) string {
	return "certs/" + domain + "-cert.pem"
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

func createCertFiles(domain string) *tls.Certificate {
	os.MkdirAll("certs", 0755)

	keyfile := _keyFile(domain)
	certfile := _certFile(domain)

	if !fileExists(keyfile) && !fileExists(certfile) {
		mkcertCmd := exec.Command("mkcert",
			"-key-file", keyfile,
			"-cert-file", certfile,
			domain)
		_, mkcertError := mkcertCmd.CombinedOutput()
		// fmt.Printf("%s\n", mkcertOutput)
		if mkcertError != nil {
			log.Print(mkcertError)
			return nil
		}
	}

	cert, err := tls.LoadX509KeyPair(_certFile(domain), _keyFile(domain))
	if err != nil {
		log.Println(err)
		return nil
	} else {
		return &cert
	}
}

func StartHTTPSServer(core *CoreServer) error {
	var certsmap sync.Map
	getOrCreateCertificate := func(domain string) *tls.Certificate {
		existing, ok := certsmap.Load(domain)
		if ok {
			return existing.(*tls.Certificate)
		} else {
			cert := createCertFiles(domain)
			if cert != nil {
				certsmap.Store(domain, cert)
			}
			return cert
		}
	}

	hs := http.Server{
		Addr:    ":https",
		Handler: core,
		TLSConfig: &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				log.Println("GetCertificate")
				log.Println(chi)
				return getOrCreateCertificate(chi.ServerName), nil
			},
		},
	}
	log.Println("starting https locally")
	// return hs.ListenAndServe()
	return hs.ListenAndServeTLS("", "")
}

func StartCoreServerHTTP(core *CoreServer) error {
	server := http.Server{Addr: ":http", Handler: core}
	return server.ListenAndServe()
}

func (s *CoreServer) Start() {
	shutdownPreviousInstance()
	go startUDPServer(s)

	if cmdInstalled("mkcert") {
		go httpToHttpsRedirector()
		log.Fatal(StartHTTPSServer(s))
	} else {
		log.Fatal(StartCoreServerHTTP(s))
	}
}

// required!
func InitLogger() {
	// nothing here in local mode!
}
