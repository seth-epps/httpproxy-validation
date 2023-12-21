package main

import (
	"fmt"
	"log/slog"
	"slices"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
)

const (
	defaultIngressclass    = "contour"
	ingressClassAnnotation = "kubernetes.io/ingress.class"
)

type Validator struct {
	Store                Store
	TargetIngressClasses []string
}

type ValidationResponse struct {
	Valid  bool
	Reason string
}

func (v Validator) IsValidProxy(proxy contourv1.HTTPProxy) (ValidationResponse, error) {
	if !v.proxyMatchesTargetIngressClasses(proxy) {
		return ValidationResponse{
			Valid: true,
		}, nil
	}
	proxies, err := v.Store.ListHTTPProxies()
	if err != nil {
		return ValidationResponse{
			Valid:  false,
			Reason: "could not list resources in cluster",
		}, err
	}

	var conflictingProxies []string

	for _, p := range proxies {
		// TODO: Handle non-root httpproxy ?
		//
		// HTTPProxy must be targetted by contour to
		// be in conflict
		if v.proxyMatchesTargetIngressClasses(p) &&
			p.Spec.VirtualHost.Fqdn == proxy.Spec.VirtualHost.Fqdn {
			conflictingProxies = append(conflictingProxies, p.Name)
		}
	}

	if len(conflictingProxies) > 0 {
		slog.Info("CONFLICT!!!", "conflicts", conflictingProxies)
		return ValidationResponse{
			Valid:  false,
			Reason: fmt.Sprintf("%s is in conflict with %v", proxy.Name, conflictingProxies),
		}, nil
	}

	return ValidationResponse{
		Valid: true,
	}, nil
}

func (v Validator) proxyMatchesTargetIngressClasses(proxy contourv1.HTTPProxy) bool {
	// First check if the ingress class annotation is set to a non-zero value since
	// it takes precendence over the spec ingress class name
	if annotationClassValue := proxy.GetAnnotations()[ingressClassAnnotation]; annotationClassValue != "" {
		return v.matchIngressClass(annotationClassValue)
	}

	return v.matchIngressClass(proxy.Spec.IngressClassName)
}

func (v Validator) matchIngressClass(className string) bool {
	// If there's no classes configured, contour will process empty ingress classes
	// or the default ingress class, "contour"
	if len(v.TargetIngressClasses) == 0 {
		return className == "" || className == defaultIngressclass
	}

	return slices.Contains(v.TargetIngressClasses, className)
}
