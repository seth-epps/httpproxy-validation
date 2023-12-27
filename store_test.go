package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	fake "k8s.io/client-go/rest/fake"
)

func TestListHTTPProxies(t *testing.T) {
	fakeClient := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
		header := http.Header{}
		header.Set("Content-Type", runtime.ContentTypeJSON)

		jsonOut := `{
	"metadata": {},
	"items": [
		{
			"metadata": {
				"name": "proxy-1",
				"creationTimestamp": null
			},
			"spec": {
				"virtualhost": {
				"fqdn": "foo.bar.com"
				},
				"ingressClassName": "contour"
			},
			"status": {
				"loadBalancer": {}
			}
		},
		{
			"metadata": {
				"name": "proxy-2",
				"creationTimestamp": null
			},
			"spec": {
				"virtualhost": {
				"fqdn": "foo.baz.com"
				}
			},
			"status": {
				"loadBalancer": {}
			}
		}
	]
}`
		return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(bytes.NewReader([]byte(jsonOut)))}, nil
	})

	c := discovery.NewDiscoveryClientForConfigOrDie(&rest.Config{})
	c.RESTClient().(*rest.RESTClient).Client = fakeClient

	p1 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "contour",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.bar.com",
			},
		},
	}
	p1.SetName("proxy-1")

	p2 := contourv1.HTTPProxy{
		Spec: contourv1.HTTPProxySpec{
			IngressClassName: "",
			VirtualHost: &contourv1.VirtualHost{
				Fqdn: "foo.baz.com",
			},
		},
	}
	p2.SetName("proxy-2")

	expected := contourv1.HTTPProxyList{Items: []contourv1.HTTPProxy{
		p1,
		p2,
	}}

	store := ClusterStore{
		c.RESTClient(),
	}

	resp, err := store.ListHTTPProxies()
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	if diff := cmp.Diff(resp, expected.Items); diff != "" {
		t.Errorf("ListHTTPProxies: (-got +want)\n%s", diff)
	}
}

func TestListHTTPProxiesError(t *testing.T) {
	expectedErr := errors.New("oh no!")
	fakeClient := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
		return nil, expectedErr
	})

	c := discovery.NewDiscoveryClientForConfigOrDie(&rest.Config{})
	c.RESTClient().(*rest.RESTClient).Client = fakeClient

	store := ClusterStore{
		c.RESTClient(),
	}

	_, err := store.ListHTTPProxies()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errString := err.Error()

	if !strings.Contains(errString, expectedErr.Error()) {
		t.Errorf("Unexpected error: \n%s", errString)
	}
}
