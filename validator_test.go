package main

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
)

type TestStore struct {
	list func() ([]contourv1.HTTPProxy, error)
}

func (ts *TestStore) ListHTTPProxies() ([]contourv1.HTTPProxy, error) {
	return ts.list()
}

func TestIsValidProxy(t *testing.T) {
	p1 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "targetted",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.bar.com",
			},
		},
	}
	p1.SetName("proxy1")

	p2 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "other-targetted",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.baz.com",
			},
		},
	}
	p2.SetName("proxy2")

	p3 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "not-targetted",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.bar.com",
			},
		},
	}
	p3.SetName("proxy3")

	p4 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "superceded-by-annotation",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "bar.foo.com",
			},
		},
	}
	p4.SetAnnotations(map[string]string{
		"kubernetes.io/ingress.class": "targetted",
	})
	p4.SetName("proxy4")

	store := &TestStore{
		list: func() ([]contourv1.HTTPProxy, error) {
			return []contourv1.HTTPProxy{
				p1,
				p2,
				p3,
				p4,
			}, nil
		},
	}

	validator := Validator{
		TargetIngressClasses: []string{"targetted", "other-targetted"},
		Store:                store,
	}

	tests := []struct {
		name     string
		proxy    contourv1.HTTPProxy
		expected ValidationResponse
	}{
		{
			"proxy ingress classes not targetted by validator",
			contourv1.HTTPProxy{
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "not-targetted",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "foo.bar.com",
					},
				},
			},
			ValidationResponse{
				Valid: true,
			},
		},
		{
			"proxy ingress classes targetted by validator, no conflict",
			contourv1.HTTPProxy{
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "targetted",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "baz.foo.com",
					},
				},
			},
			ValidationResponse{
				Valid: true,
			},
		},
		{
			"proxy ingress classes targetted by validator, conflicting fqdn",
			contourv1.HTTPProxy{
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "targetted",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "foo.baz.com",
					},
				},
			},
			ValidationResponse{
				Valid:  false,
				Reason: "proxy-under-test is in conflict with [proxy2]",
			},
		},
		{
			"proxy ingress classes targetted by validator, conflicting fqdn, ingress class overridden by annotation",
			contourv1.HTTPProxy{
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "targetted",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "bar.foo.com",
					},
				},
			},
			ValidationResponse{
				Valid:  false,
				Reason: "proxy-under-test is in conflict with [proxy4]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.proxy.SetName("proxy-under-test")
			resp, err := validator.IsValidProxy(tt.proxy)
			if err != nil {
				t.Fatalf("unexpected error validating proxy: %s", err.Error())
			}

			if diff := cmp.Diff(resp, tt.expected); diff != "" {
				t.Errorf("ValidationResponse %s: (-got +want)\n%s", tt.name, diff)
			}
		})
	}
}

func TestIsValidProxyDefaultIngressClasses(t *testing.T) {
	p1 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.bar.com",
			},
		},
	}
	p1.SetName("proxy1")

	p2 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "contour",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.baz.com",
			},
		},
	}
	p2.SetName("proxy2")

	store := &TestStore{
		list: func() ([]contourv1.HTTPProxy, error) {
			return []contourv1.HTTPProxy{
				p1,
				p2,
			}, nil
		},
	}

	validator := Validator{
		Store: store,
	}

	tests := []struct {
		name     string
		proxy    contourv1.HTTPProxy
		expected ValidationResponse
	}{
		{
			"proxy ingress classes not targetted by validator",
			contourv1.HTTPProxy{
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "not-targetted",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "foo.bar.com",
					},
				},
			},
			ValidationResponse{
				Valid: true,
			},
		},
		{
			"proxy ingress classes is empty",
			contourv1.HTTPProxy{
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "foo.bar.com",
					},
				},
			},
			ValidationResponse{
				Valid:  false,
				Reason: "proxy-under-test is in conflict with [proxy1]",
			},
		},
		{
			"proxy ingress classes is default",
			contourv1.HTTPProxy{
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "contour",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "foo.bar.com",
					},
				},
			},
			ValidationResponse{
				Valid:  false,
				Reason: "proxy-under-test is in conflict with [proxy1]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.proxy.SetName("proxy-under-test")
			resp, err := validator.IsValidProxy(tt.proxy)
			if err != nil {
				t.Fatalf("unexpected error validating proxy: %s", err.Error())
			}

			if diff := cmp.Diff(resp, tt.expected); diff != "" {
				t.Errorf("ValidationResponse %s: (-got +want)\n%s", tt.name, diff)
			}
		})
	}
}

func TestIsValidProxyStoreError(t *testing.T) {

	store := &TestStore{
		list: func() ([]contourv1.HTTPProxy, error) {
			return nil, errors.New("failed to query store")
		},
	}

	validator := Validator{
		Store: store,
	}

	proxy := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.bar.com",
			},
		},
	}

	expectedResp := ValidationResponse{
		Valid:  false,
		Reason: "could not list resources",
	}
	expectedErr := "failed to query store"

	resp, err := validator.IsValidProxy(proxy)
	if err == nil {
		t.Fatalf("expected error validating proxy, got nil")
	}

	errString := err.Error()

	if diff := cmp.Diff(errString, expectedErr); diff != "" {
		t.Errorf("Unexpected error: (-got +want)\n%s", diff)
	}

	if diff := cmp.Diff(resp, expectedResp); diff != "" {
		t.Errorf("ValidationResponse: (-got +want)\n%s", diff)
	}
}
