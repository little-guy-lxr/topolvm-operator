/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by informer-gen. DO NOT EDIT.

package v2

import (
	"context"
	time "time"

	topolvmv2 "github.com/alauda/nativestor/apis/topolvm/v2"
	versioned "github.com/alauda/nativestor/generated/nativestore/topolvm/clientset/versioned"
	internalinterfaces "github.com/alauda/nativestor/generated/nativestore/topolvm/informers/externalversions/internalinterfaces"
	v2 "github.com/alauda/nativestor/generated/nativestore/topolvm/listers/topolvm/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// TopolvmClusterInformer provides access to a shared informer and lister for
// TopolvmClusters.
type TopolvmClusterInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v2.TopolvmClusterLister
}

type topolvmClusterInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewTopolvmClusterInformer constructs a new informer for TopolvmCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewTopolvmClusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredTopolvmClusterInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredTopolvmClusterInformer constructs a new informer for TopolvmCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredTopolvmClusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.TopolvmV2().TopolvmClusters(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.TopolvmV2().TopolvmClusters(namespace).Watch(context.TODO(), options)
			},
		},
		&topolvmv2.TopolvmCluster{},
		resyncPeriod,
		indexers,
	)
}

func (f *topolvmClusterInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredTopolvmClusterInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *topolvmClusterInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&topolvmv2.TopolvmCluster{}, f.defaultInformer)
}

func (f *topolvmClusterInformer) Lister() v2.TopolvmClusterLister {
	return v2.NewTopolvmClusterLister(f.Informer().GetIndexer())
}
