package storagebackend

import (
	"context"
	"fmt"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/storage/sqlite/imitator"
	"github.com/kubeedge/kubeedge/pkg/apiserverlite"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"k8s.io/client-go/tools/cache"
	"strings"
)

//type Storage struct {
//	*REST
//}
// REST implements a RESTStorage for API services against etcd
type REST struct {
	*myStore
}
// func NewStorage(resource schema.GroupResource, optsGetter generic.RESTOptionsGetter, categories []string, tableConvertor rest.TableConvertor) CustomResourceStorage {
// 	customResourceREST, customResourceStatusREST := newREST(resource, kind, listKind, strategy, optsGetter, categories, tableConvertor)
//
// 	s := CustomResourceStorage{
// 		CustomResource: customResourceREST,
// 	}
// 	return s
// }


type myStore struct {
	*genericregistry.Store
}
// override CompleteWithOptions
func(e *myStore)CompleteWithOptions(options *generic.StoreOptions) error {
	/*
	if e.DefaultQualifiedResource.Empty() {
		return fmt.Errorf("store %#v must have a non-empty qualified resource", e)
	}
	 */
	if e.NewFunc == nil {
		return fmt.Errorf("store for %s must have NewFunc set", e.DefaultQualifiedResource.String())
	}
	if e.NewListFunc == nil {
		return fmt.Errorf("store for %s must have NewListFunc set", e.DefaultQualifiedResource.String())
	}
	if (e.KeyRootFunc == nil) != (e.KeyFunc == nil) {
		return fmt.Errorf("store for %s must set both KeyRootFunc and KeyFunc or neither", e.DefaultQualifiedResource.String())
	}

	if e.TableConvertor == nil {
		return fmt.Errorf("store for %s must set TableConvertor; rest.NewDefaultTableConvertor(e.DefaultQualifiedResource) can be used to output just name/creation time", e.DefaultQualifiedResource.String())
	}

	//var isNamespaced bool
	isNamespaced := true
	if e.CreateStrategy !=nil || e.UpdateStrategy !=nil || e.DeleteStrategy !=nil{
		return fmt.Errorf("store must have not Create Update Delete Strategy for now")
	}

	if options.RESTOptions == nil {
		return fmt.Errorf("options for %s must have RESTOptions set", e.DefaultQualifiedResource.String())
	}

	attrFunc := storage.DefaultNamespaceScopedAttr
	if e.PredicateFunc == nil {
		e.PredicateFunc = func(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
			return storage.SelectionPredicate{
				Label:    label,
				Field:    field,
				GetAttrs: attrFunc,
			}
		}
	}

	//err := validateIndexers(options.Indexers)
	//if err != nil {
	//	return err
	//}

	opts, err := options.RESTOptions.GetRESTOptions(e.DefaultQualifiedResource)
	if err != nil {
		return err
	}

	// ResourcePrefix must come from the underlying factory
	prefix := opts.ResourcePrefix
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if prefix == "/" {
		return fmt.Errorf("store for %s has an invalid prefix %q", e.DefaultQualifiedResource.String(), opts.ResourcePrefix)
	}

	// Set the default behavior for storagebackend key generation
	if e.KeyRootFunc == nil && e.KeyFunc == nil {
		if isNamespaced {
			e.KeyRootFunc = apiserverlite.KeyRootFunc
			e.KeyFunc = apiserverlite.KeyFuncReq
		} else {
			e.KeyRootFunc = func(ctx context.Context) string {
				return prefix
			}
			e.KeyFunc = func(ctx context.Context, name string) (string, error) {
				return genericregistry.NoNamespaceKeyFunc(ctx, prefix, name)
			}
		}
	}

	keyFunc := apiserverlite.KeyFuncObj

	if e.DeleteCollectionWorkers == 0 {
		e.DeleteCollectionWorkers = opts.DeleteCollectionWorkers
	}

	e.EnableGarbageCollection = opts.EnableGarbageCollection

	if e.ObjectNameFunc == nil {
		e.ObjectNameFunc = func(obj runtime.Object) (string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return "", err
			}
			return accessor.GetName(), nil
		}
	}

	if e.Storage.Storage == nil {
		e.Storage.Codec = opts.StorageConfig.Codec
		var err error
		e.Storage.Storage, e.DestroyFunc, err = opts.Decorator(
			opts.StorageConfig,
			prefix,
			keyFunc,
			e.NewFunc,
			e.NewListFunc,
			attrFunc,
			options.TriggerFunc,
			options.Indexers,
		)
		if err != nil {
			return err
		}
		e.StorageVersioner = opts.StorageConfig.EncodeVersioner
	}

	return nil

}
func newREST() (*REST) {
	deepStore := &genericregistry.Store{
		NewFunc: func() runtime.Object {
			return &unstructured.Unstructured{}
		},
		NewListFunc: func() runtime.Object {
			return &unstructured.UnstructuredList{}
		},
		PredicateFunc:           nil,   //completed by CompleteWithOptions
		DefaultQualifiedResource: schema.ParseGroupResource("unstructured"),

		CreateStrategy: nil,
		UpdateStrategy: nil,
		DeleteStrategy: nil,

		TableConvertor: rest.NewDefaultTableConvertor(schema.ParseGroupResource("unstructured")),
	}
	store := myStore{deepStore}

	var newSqliteDecorator = func(
		config *storagebackend.Config,
		resourcePrefix string,
		keyFunc func(obj runtime.Object) (string, error),
		newFunc func() runtime.Object,
		newListFunc func() runtime.Object,
		getAttrsFunc storage.AttrFunc,
		trigger storage.IndexerFuncs,
		indexers *cache.Indexers) (storage.Interface, factory.DestroyFunc, error) {
		emptyfunc := func() {}
		return mysb.New(imitator.DefaultV2Client, config.Codec, config.Transformer), emptyfunc, nil
	}
	optsGetter := generic.RESTOptions{
		StorageConfig: &storagebackend.Config{
			Type: "metamanager",
			Prefix: "",
			//Transport: nil,
			Paging: false,
			Codec: unstructured.UnstructuredJSONScheme,
			EncodeVersioner: nil,
			Transformer: nil,
			CompactionInterval:0,
			CountMetricPollPeriod:0,
			DBMetricPollInterval:0,
		},
		//Decorator: generic.UndecoratedStorage,
		Decorator: newSqliteDecorator,
		EnableGarbageCollection: false,
		DeleteCollectionWorkers : 1,
		ResourcePrefix          : "unstructured",
		CountMetricPollPeriod   :0,
	}
	GetAttrs := func(obj runtime.Object) (labels.Set, fields.Set, error){
		return storage.DefaultNamespaceScopedAttr(obj)
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}
	return &REST{&store}
}
// List returns a list of items matching labels and field according to the store's PredicateFunc.
func (r *REST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	l, err := r.Store.List(ctx, options)
	if err != nil {
		return nil, err
	}

	// Shallow copy ObjectMeta in returned list for each item. Native types have `Items []Item` fields and therefore
	// implicitly shallow copy ObjectMeta. The generic store sets the self-link for each item. So this is necessary
	// to avoid mutation of the objects from the cache.
	if ul, ok := l.(*unstructured.UnstructuredList); ok {
		for i := range ul.Items {
			shallowCopyObjectMeta(&ul.Items[i])
		}
	}

	return l, nil
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	o, err := r.Store.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	if u, ok := o.(*unstructured.Unstructured); ok {
		shallowCopyObjectMeta(u)
	}
	return o, nil
}
func (r *REST) Watch(){

}

func shallowCopyObjectMeta(u runtime.Unstructured) {
	obj := shallowMapDeepCopy(u.UnstructuredContent())
	if metadata, ok := obj["metadata"]; ok {
		if metadata, ok := metadata.(map[string]interface{}); ok {
			obj["metadata"] = shallowMapDeepCopy(metadata)
			u.SetUnstructuredContent(obj)
		}
	}
}

func shallowMapDeepCopy(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}

	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}

	return out
}

