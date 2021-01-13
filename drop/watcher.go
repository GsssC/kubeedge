package storagebackend

import (
	"context"
	"errors"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/storage/sqlite/imitator"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/dao/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/value"
	"k8s.io/klog/v2"
	"strings"
	"sync"
)

const (
	// We have set a buffer in order to reduce times of context switches.
	incomingBufSize = 100
	outgoingBufSize = 100
)

// errTestingDecode is the only error that testingDeferOnDecodeError catches during a panic
var errTestingDecode = errors.New("sentinel error only used during testing to indicate watch decoding error")

// testingDeferOnDecodeError is used during testing to recover from a panic caused by errTestingDecode, all other values continue to panic
func testingDeferOnDecodeError() {
	if r := recover(); r != nil && r != errTestingDecode {
		panic(r)
	}
}

type watcher struct {
	client      imitator.Client
	codec       runtime.Codec
	versioner   storage.Versioner
	transformer value.Transformer
}

// watchChan implements watch.Interface.
type watchChan struct {
	watcher           *watcher
	key               string
	initialRev        int64
	recursive         bool
	internalPred      storage.SelectionPredicate
	ctx               context.Context
	cancel            context.CancelFunc
	incomingEventChan chan *watch.Event
	resultChan chan watch.Event
	errChan           chan error
}
func (wc *watchChan) Receive(e watch.Event) error{
	wc.sendEvent(&e)
	return nil
}
func newWatcher(client imitator.Client, codec runtime.Codec, versioner storage.Versioner, transformer value.Transformer) *watcher {
	return &watcher{
		client:      client,
		codec:       codec,
		versioner:   versioner,
		transformer: transformer,
	}
}

// Watch watches on a key and returns a watch.Interface that transfers relevant notifications.
// If rev is zero, it will return the existing object(s) and then start watching from
// the maximum revision+1 from returned objects.
// If rev is non-zero, it will watch events happened after given revision.
// If recursive is false, it watches on given key.
// If recursive is true, it watches any children and directories under the key, excluding the root key itself.
// pred must be non-nil. Only if pred matches the change, it will be returned.
func (w *watcher) Watch(ctx context.Context, key string, rev int64, recursive bool, pred storage.SelectionPredicate) (watch.Interface, error) {
	wc := w.createWatchChan(ctx, key, rev, recursive, pred)
	go wc.run()
	return wc, nil
}

func getGVRFrom(info *apirequest.RequestInfo) schema.GroupVersionResource {
	gvr := schema.GroupVersionResource{
		Group: info.APIGroup,
		Version: info.APIVersion,
		Resource: info.Resource,
	}
	return gvr
}
func (w *watcher) createWatchChan(ctx context.Context, key string, rev int64, recursive bool, pred storage.SelectionPredicate) *watchChan {
	wc := &watchChan{
		watcher:           w,
		key:               key,
		initialRev:        rev,
		recursive:         recursive,
		internalPred:      pred,
		incomingEventChan: make(chan *watch.Event, incomingBufSize),
		resultChan:        make(chan watch.Event, outgoingBufSize),
		errChan:           make(chan error, 1),
	}
	if pred.Empty() {
		// The filter doesn't filter out any object.
		wc.internalPred = storage.Everything
	}
	wc.ctx, wc.cancel = context.WithCancel(ctx)
	return wc
}

func (wc *watchChan) run() {
	watchClosedCh := make(chan struct{})
	go wc.startWatching(watchClosedCh)

	var resultChanWG sync.WaitGroup
	resultChanWG.Add(1)
	go wc.processEvent(&resultChanWG)

	select {
	case err := <-wc.errChan:
		if err == context.Canceled {
			break
		}
		errResult := transformErrorToEvent(err)
		if errResult != nil {
			// error result is guaranteed to be received by user before closing ResultChan.
			select {
			case wc.resultChan <- *errResult:
			case <-wc.ctx.Done(): // user has given up all results
			}
		}
	case <-watchClosedCh:
	case <-wc.ctx.Done(): // user cancel
	}

	// We use wc.ctx to reap all goroutines. Under whatever condition, we should stop them all.
	// It's fine to double cancel.
	wc.cancel()

	// we need to wait until resultChan wouldn't be used anymore
	resultChanWG.Wait()
	close(wc.resultChan)
}

func (wc *watchChan) Stop() {
	wc.cancel()
}

func (wc *watchChan) ResultChan() <-chan watch.Event {
	return wc.resultChan
}

// sync tries to retrieve existing data and send them to process.
// The revision to watch will be set to the revision in response.
// All events sent will have isCreated=true
func (wc *watchChan) sync() error {
	switch wc.recursive{
	case true: /*list*/
		resp ,err :=wc.watcher.client.List(context.TODO(),wc.key)
		if err !=nil{
			return err
		}
		for _, kv := range *resp.Kvs {
			wc.sendEvent(wc.parseMeta(&kv))
		}
		wc.initialRev = int64(resp.Revision)
	case false: /*get*/
		resp ,err :=wc.watcher.client.Get(context.TODO(),wc.key)
		if err !=nil {
			return err
		}
		for _, kv := range *resp.Kvs {
			wc.sendEvent(wc.parseMeta(&kv))
		}
		wc.initialRev= int64(resp.Revision)
	}
	return nil
}

// parseMeta converts meta data to watch.Event
func (wc *watchChan)parseMeta(kv *v2.MetaV2) *watch.Event {
	obj,err:=runtime.Decode(wc.watcher.codec,[]byte(kv.Value))
	utilruntime.Must(err)
	return &watch.Event{
		Type: watch.Added,
		Object: obj,
	}
}
func getGVRFromReqInfo(info *apirequest.RequestInfo)schema.GroupVersionResource{
	gvr := schema.GroupVersionResource{
		Group: info.APIGroup,
		Version: info.APIVersion,
		Resource: info.Resource,
	}
	return gvr
}

// logWatchChannelErr checks whether the error is about mvcc revision compaction which is regarded as warning
func logWatchChannelErr(err error) {
	if !strings.Contains(err.Error(), "mvcc: required revision has been compacted") {
		klog.Errorf("watch chan error: %v", err)
	} else {
		klog.Warningf("watch chan error: %v", err)
	}
}

// startWatching does:
// - get current objects if initialRev=0; set initialRev to current rev
// - watch on given key and send events to process.
func (wc *watchChan) startWatching(watchClosedCh chan struct{}) {
	if wc.initialRev == 0 {
		if err := wc.sync(); err != nil {
			klog.Errorf("failed to sync with latest state: %v", err)
			wc.sendError(err)
			return
		}
	}
	wch := wc.watcher.client.Watch(wc.ctx, wc.key,uint64(wc.initialRev+1))
	for wres := range wch {
		wc.sendEvent(&wres)
	}
	close(watchClosedCh)
}

// processEvent processes events from etcd watcher and sends results to resultChan.
func (wc *watchChan) processEvent(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case e := <-wc.incomingEventChan:
			if wc.filter(e.Object) {
				continue
			}
			if len(wc.resultChan) == outgoingBufSize {
				klog.V(3).InfoS("Fast watcher, slow processing. Probably caused by slow dispatching events to watchers", "outgoingEvents", outgoingBufSize)
			}
			// If user couldn't receive results fast enough, we also block incoming events from watcher.
			// Because storing events in local will cause more memory usage.
			// The worst case would be closing the fast watcher.
			select {
			case wc.resultChan <- *e:
			case <-wc.ctx.Done():
				return
			}
		case <-wc.ctx.Done():
			return
		}
	}
}

func (wc *watchChan) filter(obj runtime.Object) bool {
	if wc.internalPred.Empty() {
		return true
	}
	matched, err := wc.internalPred.Matches(obj)
	return err == nil && matched
}

func (wc *watchChan) acceptAll() bool {
	return wc.internalPred.Empty()
}

func transformErrorToEvent(err error) *watch.Event {
	err = apierrors.NewInternalError(err)
	status := err.(apierrors.APIStatus).Status()
	return &watch.Event{
		Type:   watch.Error,
		Object: &status,
	}
}

func (wc *watchChan) sendError(err error) {
	select {
	case wc.errChan <- err:
	case <-wc.ctx.Done():
	}
}

func (wc *watchChan) sendEvent(e *watch.Event) {
	if len(wc.incomingEventChan) == incomingBufSize {
		klog.V(3).InfoS("Fast watcher, slow processing. Probably caused by slow decoding, user not receiving fast, or other processing logic", "incomingEvents", incomingBufSize)
	}
	select {
	case wc.incomingEventChan <- e:
	case <-wc.ctx.Done():
	}
}
