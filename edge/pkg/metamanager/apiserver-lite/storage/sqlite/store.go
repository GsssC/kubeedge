package sqlite

import (
	"context"
	"fmt"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/storage/sqlite/imitator"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/klog/v2"
	"reflect"
	"strconv"
)

/*
	TODO:
	此文件旨在将imitator封装成store.Interface
	借用dao/v2/client.go访问后端sqlite
*/
type store struct {
	//GsssC struct field
	client    imitator.Client
	versioner storage.Versioner
	// to co/decoder obj
	codec runtime.Codec
	watcher *watcher


	//Shao struct field
}

func (s *store) Versioner() storage.Versioner {
	return s.versioner
}

func (s *store) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	panic("Do not call this function")
}

func (s *store) Delete(ctx context.Context, key string, out runtime.Object, preconditions *storage.Preconditions, validateDeletion storage.ValidateObjectFunc) error {
	panic("Do not call this function")
}

func (s *store) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	return s.watch(ctx, key, opts, false)
}

func (s *store) WatchList(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	return s.watch(ctx, key, opts, true)
}
func (s *store) watch(ctx context.Context, key string, opts storage.ListOptions, recursive bool) (watch.Interface, error) {
	rev, err := s.versioner.ParseResourceVersion(opts.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return s.watcher.Watch(ctx, key, int64(rev), recursive, opts.Predicate)
}

//TODO:Shao
func (s *store) Get(ctx context.Context, key string, opts storage.GetOptions, objPtr runtime.Object) error {
	panic("Get implement me")
}

//TODO:Shao
func (s *store) GetToList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	panic("GetToList implement me")
}

//TODO:Shao
func (s *store) List(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	klog.Infof("get a list req, key:=%v",key)
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return fmt.Errorf("need ptr to slice: %v", err)
	}

	resp,err := imitator.DefaultV2Client.List(context.TODO(),key)

	if err !=nil || len(*resp.Kvs) == 0{
		//responsewriters.ErrorNegotiated(err,ls.NegotiatedSerializer,gv,w,req)
		klog.Error(err)
		return err
	}
	var unstrList *unstructured.UnstructuredList
	unstrList = listObj.(*unstructured.UnstructuredList)
	for _,v := range *resp.Kvs {
		var unstrObj unstructured.Unstructured
		_,_,_= s.codec.Decode([]byte(v.Value),nil,&unstrObj)
		unstrList.Items=append(unstrList.Items, unstrObj)
	}
	rv := strconv.FormatUint(resp.Revision,10)
	unstrList.SetResourceVersion(rv)
	//unstrList.SetSelfLink(key)
	//unstrList.SetGroupVersionKind(gv.WithKind(v2.UnsafeResourceToKind(gvr.Resource)+"List"))
	return nil
}

func (s *store) GuaranteedUpdate(ctx context.Context, key string, ptrToType runtime.Object, ignoreNotFound bool, precondtions *storage.Preconditions, tryUpdate storage.UpdateFunc, suggestion ...runtime.Object) error {
	panic("Do not call this function")
}

func (s *store) Count(key string) (int64, error) {
	panic("implement me")
}

func New()storage.Interface{
	return newStore()
}
func newStore()*store {
	codec := unstructured.UnstructuredJSONScheme
	client := imitator.DefaultV2Client
	s := store{
		client:    client,
		versioner: imitator.Versioner,
		watcher:   NewWatcher(client,codec),
		codec :    codec,
	}
	return &s
}