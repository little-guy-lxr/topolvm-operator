package controller

import (
	topolvmv2 "github.com/alauda/topolvm-operator/apis/topolvm/v2"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func predicateController() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if _, ok := e.Object.(*topolvmv2.TopolvmCluster); ok {
				return true
			}
			return false
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			if old, ok := e.ObjectOld.(*topolvmv2.TopolvmCluster); ok {
				if new, ok := e.ObjectNew.(*topolvmv2.TopolvmCluster); ok {
					return !reflect.DeepEqual(old.Spec, new.Spec)
				}
			}
			return false
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			if _, ok := e.Object.(*topolvmv2.TopolvmCluster); ok {
				return true
			}
			return false
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
