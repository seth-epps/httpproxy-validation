package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type serverConfig struct {
	tlsKeyPath  string
	tlsCertPath string
	port        int
}

func main() {
	var server serverConfig
	var ingressClasses string
	flag.StringVar(&server.tlsKeyPath, "tls-key", "/etc/certs/tls.key", "Path to the TLS key")
	flag.StringVar(&server.tlsCertPath, "tls-cert", "/etc/certs/tls.crt", "Path to the TLS certificate")
	flag.IntVar(&server.port, "port", 8443, "Server port")
	flag.StringVar(&ingressClasses, "ingress-classes", "", "Comma separated list of ingress class names to validate against")
	flag.Parse()

	k8sStore, err := NewInClusterStore()
	if err != nil {
		slog.Error("Failed to setup cluster store", "error", err.Error())
		os.Exit(1)
	}

	httpProxyValidator := Validator{
		Store:                k8sStore,
		TargetIngressClasses: strings.Split(ingressClasses, ","),
	}

	if err := run(server, httpProxyValidator); err != nil {
		slog.Error("Server exited.", "error", err.Error())
		os.Exit(1)
	}
}

func run(serverConfig serverConfig, validator Validator) error {
	admissionHandler := HTTPProxyAdmissionHandler{
		Validator: validator,
	}

	mux := http.NewServeMux()
	mux.Handle("/validate", AdmissionMiddleware(admissionHandler.Validate))

	addr := fmt.Sprintf(":%d", serverConfig.port)
	slog.Info("Server starting", "addr", addr)

	return http.ListenAndServeTLS(addr, serverConfig.tlsCertPath, serverConfig.tlsKeyPath, mux)
}
