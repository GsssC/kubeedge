/*
Copyright 2016 The Kubernetes Authors.

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

package storagebackend

import (
	"context"
	"fmt"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/storage/sqlite/imitator"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/etcd3"
	"k8s.io/apiserver/pkg/storage/value"
	"k8s.io/klog/v2"
	"reflect"
)


type myStore struct{
	client      imitator.Client
	codec       runtime.Codec
	versioner   storage.Versioner
	transformer value.Transformer
	watcher     *watcher
}

// New returns an metastore implementation of storagebackend.Interface.
func New(c imitator.Client, codec runtime.Codec, transformer value.Transformer) storage.Interface {
	return newStore(c, codec,transformer)
}

func newStore(c imitator.Client,  codec runtime.Codec, transformer value.Transformer) *myStore {
	versioner := etcd3.APIObjectVersioner{}
	result := &myStore{
		client:        c,
		codec:         codec,
		versioner:     versioner,
		transformer:   transformer,
		watcher:      newWatcher(c, codec, versioner, transformer),
	}
	return result
}

// Versioner implements storagebackend.Interface.Versioner.
func (s *myStore) Versioner() storage.Versioner {
	return s.versioner
}

// Get implements storagebackend.Interface.Get.
func (s *myStore) Get(ctx context.Context, key string, opts storage.GetOptions, out runtime.Object) error {
	getResp, err := s.client.Get(ctx, key)
	if err != nil {
		return err
	}
	if err = s.validateMinimumResourceVersion(opts.ResourceVersion, uint64(getResp.Revision)); err != nil {
		return err
	}

	if len(*getResp.Kvs) == 0 {
		if opts.IgnoreNotFound {
			return runtime.SetZeroValue(out)
		}
		return storage.NewKeyNotFoundError(key, 0)
	}
	kv := (*getResp.Kvs)[0]

	return decode(s.codec, s.versioner, []byte(kv.Value),out, -1)
}

// Create implements storagebackend.Interface.Create.
func (s *myStore) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	return nil
}

// Delete implements storagebackend.Interface.Delete.
func (s *myStore) Delete(ctx context.Context, key string, out runtime.Object, preconditions *storage.Preconditions, validateDeletion storage.ValidateObjectFunc) error {
	return nil
}

// GuaranteedUpdate implements storagebackend.Interface.GuaranteedUpdate.
func (s *myStore) GuaranteedUpdate(
	ctx context.Context, key string, out runtime.Object, ignoreNotFound bool,
	preconditions *storage.Preconditions, tryUpdate storage.UpdateFunc, suggestion ...runtime.Object) error {
	return nil
}

// GetToList implements storagebackend.Interface.GetToList.
// Because sqlite now do not cache history of resource object, so we get the most recent obj directly and ignore resourceversion match.
func (s *myStore) GetToList(ctx context.Context, key string, listOpts storage.ListOptions, listObj runtime.Object) error {

	resourceVersion := listOpts.ResourceVersion
	pred := listOpts.Predicate
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return fmt.Errorf("need ptr to slice: %v", err)
	}

	newItemFunc := getNewItemFunc(listObj, v)

	getResp, err := s.client.List(ctx, key)
	if err != nil {
		return err
	}
	if err = s.validateMinimumResourceVersion(resourceVersion,getResp.Revision); err != nil {
		return err
	}

	if len(*getResp.Kvs) > 0 {
		if err := appendListItem(v, []byte((*getResp.Kvs)[0].Value), pred, s.codec, newItemFunc); err != nil {
			return err
		}
	}
	// update version with cluster level revision
	return s.versioner.UpdateList(listObj, getResp.Revision, "", nil)
}

func getNewItemFunc(listObj runtime.Object, v reflect.Value) func() runtime.Object {
	// For unstructured lists with a target group/version, preserve the group/version in the instantiated list items
	if unstructuredList, isUnstructured := listObj.(*unstructured.UnstructuredList); isUnstructured {
		if apiVersion := unstructuredList.GetAPIVersion(); len(apiVersion) > 0 {
			return func() runtime.Object {
				return &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": apiVersion}}
			}
		}
	}

	// Otherwise just instantiate an empty item
	elem := v.Type().Elem()
	return func() runtime.Object {
		return reflect.New(elem).Interface().(runtime.Object)
	}
}

func (s *myStore) Count(key string) (int64, error) {
	getResp, err := s.client.Get(context.Background(),key)
	if err != nil {
		return 0, err
	}
	return int64(len(*getResp.Kvs)), nil
}


// List implements storagebackend.Interface.List.
// Because sqlite now do not cache history of resource object, so we get the single and most rest recent obj directly.
func (s *myStore) List(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	resourceVersion := opts.ResourceVersion
	pred := opts.Predicate
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return fmt.Errorf("need ptr to slice: %v", err)
	}

	newItemFunc := getNewItemFunc(listObj, v)

	var returnedRV uint64
	var getResp imitator.Resp
	if err = s.validateMinimumResourceVersion(resourceVersion, s.client.GetRevision()); err != nil {
		return err
	}
	getResp, err = s.client.List(ctx, key)
	if err !=nil{
		return nil
	}
	if len(*(getResp.Kvs)) == 0 {
		return fmt.Errorf("no results were found, but etcd indicated there were more values remaining")
	}

	// avoid small allocations for the result slice, since this can be called in many
	// different contexts and we don't know how significantly the result will be filtered
	if pred.Empty() {
		growSlice(v, len(*getResp.Kvs))
	} else {
		growSlice(v, 2048, len(*getResp.Kvs))
	}

	// take items from the response until the bucket is full, filtering as we go
	for _, kv := range *getResp.Kvs {
		if err := appendListItem(v, []byte(kv.Value), pred, s.codec, newItemFunc); err != nil {
																						  return err
																						  }
	}

	// indicate to the client which resource version was returned
	if returnedRV == 0 {
		returnedRV = getResp.Revision
	}
	return s.versioner.UpdateList(listObj, returnedRV, "", nil)
}

// growSlice takes a slice value and grows its capacity up
// to the maximum of the passed sizes or maxCapacity, whichever
// is smaller. Above maxCapacity decisions about allocation are left
// to the Go runtime on append. This allows a caller to make an
// educated guess about the potential size of the total list while
// still avoiding overly aggressive initial allocation. If sizes
// is empty maxCapacity will be used as the size to grow.
func growSlice(v reflect.Value, maxCapacity int, sizes ...int) {
	cap := v.Cap()
	max := cap
	for _, size := range sizes {
		if size > max {
			max = size
		}
	}
	if len(sizes) == 0 || max > maxCapacity {
		max = maxCapacity
	}
	if max <= cap {
		return
	}
	if v.Len() > 0 {
		extra := reflect.MakeSlice(v.Type(), 0, max)
		reflect.Copy(extra, v)
		v.Set(extra)
	} else {
		extra := reflect.MakeSlice(v.Type(), 0, max)
		v.Set(extra)
	}
}

// Watch implements storagebackend.Interface.Watch.
func (s *myStore) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	return s.watch(ctx, key, opts, false)
}

// WatchList implements storagebackend.Interface.WatchList.
func (s *myStore) WatchList(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	return s.watch(ctx, key, opts, true)
}

func (s *myStore) watch(ctx context.Context, key string, opts storage.ListOptions, recursive bool) (watch.Interface, error) {
	rev, err := s.versioner.ParseResourceVersion(opts.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return s.watcher.Watch(ctx, key, int64(rev), recursive, opts.Predicate)
}

// validateMinimumResourceVersion returns a 'too large resource' version error when the provided minimumResourceVersion is
// greater than the most recent actualRevision available from storagebackend.
func (s *myStore) validateMinimumResourceVersion(minimumResourceVersion string, actualRevision uint64) error {
	if minimumResourceVersion == "" {
		return nil
	}
	minimumRV, err := s.versioner.ParseResourceVersion(minimumResourceVersion)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("invalid resource version: %v", err))
	}
	// Enforce the storagebackend.Interface guarantee that the resource version of the returned data
	// "will be at least 'resourceVersion'".
	if minimumRV > actualRevision {
		return storage.NewTooLargeResourceVersionError(minimumRV, actualRevision, 0)
	}
	return nil
}

// decode decodes value of bytes into object. It will also set the object resource version to rev.
// On success, objPtr would be set to the object.
func decode(codec runtime.Codec, versioner storage.Versioner, value []byte, objPtr runtime.Object, rev int64) error {
	if _, err := conversion.EnforcePtr(objPtr); err != nil {
		return fmt.Errorf("unable to convert output object to pointer: %v", err)
	}
	_, _, err := codec.Decode(value, nil, objPtr)
	if err != nil {
		return err
	}
	// skip update object's resource version if rev < 0
	if rev < 0 {
		return nil
	}
	// being unable to set the version does not prevent the object from being extracted
	if err := versioner.UpdateObject(objPtr, uint64(rev)); err != nil {
		klog.Errorf("failed to update object version: %v", err)
	}
	return nil
}

// appendListItem decodes and appends the object (if it passes filter) to v, which must be a slice.
func appendListItem(v reflect.Value, data []byte, pred storage.SelectionPredicate, codec runtime.Codec, newItemFunc func() runtime.Object) error {
	obj, _, err := codec.Decode(data, nil, newItemFunc())
	if err != nil {
		return err
	}
	/* skip update obj resource version when get or list, update it when create/update whole obj
	if err := versioner.UpdateObject(obj, rev); err != nil {
		klog.Errorf("failed to update object version: %v", err)
	}
	 */
	if matched, err := pred.Matches(obj); err == nil && matched {
		v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
	}
	return nil
}

