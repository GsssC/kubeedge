package storage

import (
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/kubernetes/storage/cacher"
	"github.com/kubeedge/kubeedge/pkg/apiserverlite"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"
)

// REST implements a RESTStorage for pod presets against etcd.
type REST struct {
	*genericregistry.Store
}

// NewREST returns a RESTStorage object that will work against pod presets.
func NewREST() (*REST, error) {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &unstructured.Unstructured{} },
		NewListFunc:              func() runtime.Object { return &unstructured.UnstructuredList{} },
		DefaultQualifiedResource: schema.GroupResource{},

		KeyFunc: apiserverlite.KeyFuncReq,
		KeyRootFunc: apiserverlite.KeyRootFunc,

		CreateStrategy: nil,
		UpdateStrategy: nil,
		DeleteStrategy: nil,

		TableConvertor: nil,
		StorageVersioner : nil,
		Storage: genericregistry.DryRunnableStorage{},
	}
	store.PredicateFunc = func(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
		return storage.SelectionPredicate{
			Label:    label,
			Field:    field,
			GetAttrs: storage.DefaultNamespaceScopedAttr,
		}
	}
	Storage,err:= cacher.NewCacher()
	utilruntime.Must(err)
	store.Storage.Storage = Storage
	store.Storage.Codec = unstructured.UnstructuredJSONScheme

	return &REST{store}, nil
}
