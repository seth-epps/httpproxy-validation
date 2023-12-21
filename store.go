package main

import (
	"context"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Store interface {
	ListHTTPProxies() ([]contourv1.HTTPProxy, error)
}

type ClusterStore struct {
	client *kubernetes.Clientset
}

func NewInClusterStore() (*ClusterStore, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return NewClusterStore(config)
}

func NewClusterStore(config *rest.Config) (*ClusterStore, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &ClusterStore{clientset}, nil
}

func (cs *ClusterStore) ListHTTPProxies() ([]contourv1.HTTPProxy, error) {
	var proxyList contourv1.HTTPProxyList

	err := cs.client.
		RESTClient().
		Get().
		AbsPath("/apis/projectcontour.io/v1/httpproxies").
		Do(context.TODO()).
		Into(&proxyList)

	if err != nil {
		return nil, err
	}
	return proxyList.Items, nil
}
