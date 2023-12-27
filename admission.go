package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	admissionv1 "k8s.io/api/admission/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

var (
	runtimeScheme = runtime.NewScheme()
	serializer    = json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		runtimeScheme,
		runtimeScheme,
		json.SerializerOptions{
			Pretty: true,
		},
	)
	httpProxyResource = metav1.GroupVersionKind{
		Group:   "projectcontour.io",
		Version: "v1",
		Kind:    "HTTPProxy",
	}
)

func init() {
	_ = admissionv1.AddToScheme(runtimeScheme)
	_ = contourv1.AddToScheme(runtimeScheme)
}

type Review func(*admissionv1.AdmissionReview)

func AdmissionMiddleware(review Review) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		data, err := io.ReadAll(req.Body)
		if err != nil {
			slog.Error("Failed to read request body", "error", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		obj, _, err := serializer.Decode(data, nil, nil)
		if err != nil {
			slog.Error("Could not decode request body", "error", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		admissionReview, ok := obj.(*admissionv1.AdmissionReview)
		if !ok {
			slog.Error("Request was not an AdmissionReview", "type", fmt.Sprintf("%T", obj))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		review(admissionReview)

		err = serializer.Encode(admissionReview, w)
		if err != nil {
			slog.Error("Failed to encode response", "error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

type HTTPProxyAdmissionHandler struct {
	Validator Validator
}

func (ah *HTTPProxyAdmissionHandler) Validate(review *admissionv1.AdmissionReview) {
	resp := &admissionv1.AdmissionResponse{}
	resp.UID = review.Request.UID
	review.Response = resp

	if !apiequality.Semantic.DeepEqual(review.Request.Kind, httpProxyResource) {
		slog.Error("Review is not for HTTPProxy resource", "kind", review.Request.Kind.String(), "name", review.Request.Name)
		resp.Allowed = false
		resp.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusBadRequest,
			Reason:  metav1.StatusReasonBadRequest,
			Message: fmt.Sprintf("Review is not for HTTPProxy resource. Instead got %s with name %s", review.Request.Kind.String(), review.Request.Name),
		}
		return
	}

	raw := review.Request.Object.Raw
	proxy := contourv1.HTTPProxy{}
	if _, _, err := serializer.Decode(raw, nil, &proxy); err != nil {
		slog.Error("Failed to decode HTTPProxy", "error", err.Error())
		resp.Allowed = false
		resp.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusInternalServerError,
			Reason:  metav1.StatusReasonInternalError,
			Message: err.Error(),
		}
		return
	}

	validationResponse, err := ah.Validator.IsValidProxy(proxy)
	if err != nil {
		slog.Error("Failed to validate HTTPProxy", "error", err.Error())
		resp.Allowed = false
		resp.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusInternalServerError,
			Reason:  metav1.StatusReasonInternalError,
			Message: err.Error(),
		}
		return
	}

	if !validationResponse.Valid {
		resp.Allowed = false
		resp.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusBadRequest,
			Reason:  metav1.StatusReasonBadRequest,
			Message: validationResponse.Reason,
		}
		return
	}
	resp.Allowed = true
}
