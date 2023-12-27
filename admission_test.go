package main

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	// "github.com/google/go-cmp/cmp/cmpopts"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestHTTPProxyAdmissionHandler(t *testing.T) {
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

	handler := HTTPProxyAdmissionHandler{
		Validator: validator,
	}

	tests := []struct {
		name         string
		review       *admissionv1.AdmissionReview
		expectedResp admissionv1.AdmissionResponse
	}{
		{
			"not review for HTTPProxy",
			&admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
					Name: "my-deployment",
				},
			},
			admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Reason:  metav1.StatusReasonBadRequest,
					Message: "Review is not for HTTPProxy resource. Instead got apps/v1, Kind=Deployment with name my-deployment",
				},
			},
		},
		{
			"unable to decode review body",
			&admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{Group: "projectcontour.io", Version: "v1", Kind: "HTTPProxy"},
					Name: "my-proxy",
					Object: runtime.RawExtension{
						Raw: []byte("garbage request body"),
					},
				},
			},
			admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusInternalServerError,
					Reason:  metav1.StatusReasonInternalError,
					Message: "couldn't get version/kind; json parse error: invalid character 'g' looking for beginning of value",
				},
			},
		},
		{
			"fail validation",
			&admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{Group: "projectcontour.io", Version: "v1", Kind: "HTTPProxy"},
					Name: "proxy-new",
					Object: runtime.RawExtension{
						Raw: []byte(`
{
	"metadata": {
		"name": "proxy-new"
	},
	"spec": {
		"virtualhost": {
		"fqdn": "foo.bar.com"
		},
		"ingressClassName": "targetted"
	},
	"status": {
		"loadBalancer": {}
	}
}`),
					},
				},
			},
			admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Reason:  metav1.StatusReasonBadRequest,
					Message: "proxy-new is in conflict with [proxy1]",
				},
			},
		},
		{
			"pass validation",
			&admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{Group: "projectcontour.io", Version: "v1", Kind: "HTTPProxy"},
					Name: "proxy-new",
					Object: runtime.RawExtension{
						Raw: []byte(`
{
	"metadata": {
		"name": "proxy-new"
	},
	"spec": {
		"virtualhost": {
		"fqdn": "foo.bar2.com"
		},
		"ingressClassName": "targetted"
	},
	"status": {
		"loadBalancer": {}
	}
}`),
					},
				},
			},
			admissionv1.AdmissionResponse{
				Allowed: true,
				Result:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler.Validate(tt.review)

			if tt.review.Response.Allowed != tt.expectedResp.Allowed {
				t.Errorf("HTTPProxyAdmissionHandler.Validate %s: AdmissionResponse.Allowed got: %t,  want %t", tt.name, tt.review.Response.Allowed, tt.expectedResp.Allowed)
			}

			if diff := cmp.Diff(tt.review.Response.Result, tt.expectedResp.Result); diff != "" {
				t.Errorf("HTTPProxyAdmissionHandler.Validate %s: (-got +want)\n%s", tt.name, diff)
			}
		})
	}
}
