//go:build release

package core_server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"gopkg.in/natefinch/lumberjack.v2"
)

var UnknownDomain = errors.New("Unknown Domain")

func (s *CoreServer) AutoCertHostPolicy(_ context.Context, domain string) error {
	for _, target := range s.Targets {
		if target.Domain == domain {
			return nil
		}
	}
	return UnknownDomain
}

func StartHTTPSServer(s *CoreServer) {
	home, _ := os.UserHomeDir()
	certsDir := path.Join(home, "certs")

	certManager := &autocert.Manager{
		Cache:      autocert.DirCache(certsDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: s.AutoCertHostPolicy,
	}
	// listener := certManager.Listener()
	// http.Serve(listener, s)
	server := http.Server{
		Addr:      ":https",
		TLSConfig: certManager.TLSConfig(),
		Handler:   s,
	}
	server.ListenAndServeTLS("", "")
}

func (s *CoreServer) Start() {
	shutdownPreviousInstance()
	go startUDPServer(s)

	go httpToHttpsRedirector()
	StartHTTPSServer(s)
}

func InitLogger() {
	logger := &lumberjack.Logger{
		Filename:   "logs/core.log",
		MaxSize:    200, // megabytes
		MaxBackups: 10,
		MaxAge:     10, //days
		LocalTime:  true,
		// Compress:   true, // disabled by default
	}

	// Rotate logger on the 00:00 hour mark every day
	go func() {
		for {
			now := time.Now()
			nextDay := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
			duration := nextDay.Sub(now)
			time.Sleep(duration)
			logger.Rotate()
		}
	}()

	log.SetOutput(logger)
}
