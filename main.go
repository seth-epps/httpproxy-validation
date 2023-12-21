package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

func main() {
	var tlsKey, tlsCert, ingressClasses string
	flag.StringVar(&tlsKey, "tls-key", "/etc/certs/tls.key", "Path to the TLS key")
	flag.StringVar(&tlsCert, "tls-cert", "/etc/certs/tls.crt", "Path to the TLS certificate")
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

	admissionHandler := HTTPProxyAdmissionHandler{
		Validator: httpProxyValidator,
	}

	http.Handle("/validate", AdmissionMiddleware(admissionHandler.Validate))
	slog.Info("Server starting...")
	if err := http.ListenAndServeTLS(":8443", tlsCert, tlsKey, nil); err != nil {
		slog.Error("Server exited.", "error", err.Error())
		os.Exit(1)
	}
}
