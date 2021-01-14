package apiserver_lite

import (
	"context"
	_ "context"
	"fmt"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/kubernetes/serializer"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/storage/cacher"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/storage/sqlite"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/storage/sqlite/imitator"
	"github.com/kubeedge/kubeedge/pkg/apiserverlite"
	"github.com/kubeedge/kubeedge/pkg/apiserverlite/util"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metainternalversionscheme "k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwaitgroup "k8s.io/apimachinery/pkg/util/waitgroup"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/klog/v2"
	"net/http"
	"strconv"
	"time"
)

const(
	httpaddr = "0.0.0.0:10010"
)
// TODO: is it necessary to construct a new struct rather than server.GenericAPIServer ?
// LiteServer is simplification of server.GenericAPIServer
type LiteServer struct{
	HandlerChainWaitGroup *utilwaitgroup.SafeWaitGroup
	LongRunningFunc apirequest.LongRunningRequestCheck
	RequestTimeout time.Duration
	legacyAPIGroupPrefixes sets.String
	Handler http.Handler
	NegotiatedSerializer runtime.NegotiatedSerializer
	Storage storage.Interface
	Cacher storage.Interface
}

func NewLiteServer() *LiteServer {
	cacher,err := cacher.NewCacher()
	if err!=nil{
		klog.Errorf("failed to create cacher:%v",err)
	}
	ls := LiteServer{
		HandlerChainWaitGroup:  new(utilwaitgroup.SafeWaitGroup),
		LongRunningFunc:        genericfilters.BasicLongRunningRequestCheck(sets.NewString("watch"), sets.NewString()),
		legacyAPIGroupPrefixes: sets.NewString(server.DefaultLegacyAPIPrefix),
		NegotiatedSerializer:   serializer.NewNegotiatedSerializer(),
		Storage:                sqlite.New(),
		Cacher:                 cacher,
	}
	return &ls
}

func (ls *LiteServer)Start(stopChan <-chan struct{}){
	h := ls.BuildBasicHandler()
	h = BuildHandlerChain(h,ls)
	s := http.Server{
		Addr:    httpaddr,
		Handler: h,
	}
	utilruntime.HandleError(s.ListenAndServe())
	<-stopChan
}

func(ls *LiteServer)BuildBasicHandler()http.Handler{
	h:= http.HandlerFunc(func(w http.ResponseWriter, req *http.Request){
		ctx := req.Context()
		reqInfo , ok := apirequest.RequestInfoFrom(ctx)
		klog.Infof("[apiserver-lite]get a req:%+v",reqInfo)
		if ok && reqInfo.IsResourceRequest{
			switch{
			case reqInfo.Verb == "get":
				ls.cacherRead(w,req)
				//ls.processRead(w,req)
			case reqInfo.Verb == "list":
				ls.cacherList(w,req)
				//ls.processList(w,req)
			case reqInfo.Verb == "watch":
				ls.cacherWatch(w,req)
				//ls.processWatch(w,req)
			default:
				ls.processFail(w,req)
			}
		} else{
			responsewriters.InternalError(w,req,fmt.Errorf("not a resource request"))
		}

	})
	return h
}

// shao
func(ls *LiteServer)cacherRead(w http.ResponseWriter,req *http.Request){
	info , _ := apirequest.RequestInfoFrom(req.Context())
	key,err:= apiserverlite.KeyFuncReq(req.Context(),"")
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	gv := schema.GroupVersion{
		Group: info.APIGroup,
		Version: info.APIVersion,
	}
	unstrObj := new(unstructured.Unstructured)
	// ???
	opts := storage.GetOptions{ResourceVersion:"0",IgnoreNotFound: true}
	err = ls.Cacher.Get(req.Context(),key,opts,unstrObj)
	if err !=nil{
		klog.Error(err)
	}
	unstrObj.SetSelfLink(info.Path)
	unstrObj.SetGroupVersionKind(gv.WithKind(util.UnsafeResourceToKind(info.Resource)+"Get"))
	responsewriters.WriteObjectNegotiated(ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions,gv,w,req,http.StatusOK,unstrObj)

}



func(ls *LiteServer)cacherWatch(w http.ResponseWriter,req *http.Request){
	key,err:= apiserverlite.KeyFuncReq(req.Context(),"")
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	opts := metainternalversion.ListOptions{}
	if err := metainternalversionscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, &opts); err != nil {
		responsewriters.InternalError(w,req,err)
		klog.Error(err)
		return
	}
	PredicateFunc := func(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
		return storage.SelectionPredicate{
			Label:    label,
			Field:    field,
			GetAttrs: storage.DefaultNamespaceScopedAttr,
		}
	}
	storageOpts := storage.ListOptions{ResourceVersion: opts.ResourceVersion, ResourceVersionMatch: opts.ResourceVersionMatch,Predicate: PredicateFunc(labels.Everything(),fields.Everything())}
	watcher,err := ls.Cacher.WatchList(req.Context(),key,storageOpts)
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	//mdeiaType,_,err := negotiation.NegotiateOutputMediaType(req,ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions)
	serializerInfo,err := negotiation.NegotiateOutputMediaTypeStream(req,ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions)
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		err := fmt.Errorf("unable to start watch - can't get http.Flusher: %#v", w)
		utilruntime.HandleError(err)
		responsewriters.InternalError(w,req,fmt.Errorf("failed to get http.Flusher"))
	}
	framer := serializerInfo.StreamSerializer.Framer.NewFrameWriter(w)
	streamSerializer := serializerInfo.StreamSerializer.Serializer
	encoder := ls.NegotiatedSerializer.EncoderForVersion(streamSerializer,nil)
	//useTextFraming := serializerInfo.EncodesAsText
	//mediaType := serializerInfo.MediaType
	e := streaming.NewEncoder(framer, encoder)
	mediaType := serializerInfo.MediaType
	if mediaType != runtime.ContentTypeJSON {
		mediaType += ";stream=watch"
	}

	// begin the stream
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	internalEvent := &metav1.InternalEvent{}
	outEvent := &metav1.WatchEvent{}
	ch := watcher.ResultChan()
	neverExitWatch := make(<-chan time.Time)
	for{
		select{
		case <-req.Context().Done():
			return
		case <-neverExitWatch:
			responsewriters.InternalError(w,req,fmt.Errorf("timeout to get event from watcher"))
			klog.Errorf("timeout for watching")
			return
		case event, ok := <-ch:
			if !ok {
				responsewriters.InternalError(w,req,fmt.Errorf("failed to get event from watcher"))
				return
			}
			*outEvent = metav1.WatchEvent{}

			// create the external type directly and encode it.  Clients will only recognize the serialization we provide.
			// The internal event is being reused, not reallocated so its just a few extra assignments to do it this way
			// and we get the benefit of using conversion functions which already have to stay in sync
			*internalEvent = metav1.InternalEvent(event)
			err := metav1.Convert_v1_InternalEvent_To_v1_WatchEvent(internalEvent, outEvent, nil)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to convert watch object: %v", err))
				// client disconnect.
				return
			}
			if err := e.Encode(outEvent); err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to encode watch object %T: %v (%#v)", outEvent, err, e))
				// client disconnect.
				return
			}
			if len(ch) == 0 {
				flusher.Flush()
			}

		}
	}

}
func(ls *LiteServer)cacherList(w http.ResponseWriter,req *http.Request){
	info , _ := apirequest.RequestInfoFrom(req.Context())
	key,err:= apiserverlite.KeyFuncReq(req.Context(),"")
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	gv := schema.GroupVersion{
		Group: info.APIGroup,
		Version: info.APIVersion,
	}
	unstrList := new(unstructured.UnstructuredList)
	opts := storage.ListOptions{ResourceVersion:"0",Predicate: storage.Everything}
	err = ls.Cacher.List(req.Context(),key,opts,unstrList)
	if err !=nil{
		klog.Error(err)
	}
	unstrList.SetSelfLink(info.Path)
	unstrList.SetGroupVersionKind(gv.WithKind(util.UnsafeResourceToKind(info.Resource)+"List"))
	responsewriters.WriteObjectNegotiated(ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions,gv,w,req,http.StatusOK,unstrList)

}

func(ls *LiteServer)processList(w http.ResponseWriter,req *http.Request){
	info , _ := apirequest.RequestInfoFrom(req.Context())
	key,err:= apiserverlite.KeyFuncReq(req.Context(),"")
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	resp,err := imitator.DefaultV2Client.List(context.TODO(),key)
	gv := schema.GroupVersion{
		Group: info.APIGroup,
		Version: info.APIVersion,
	}
	if err !=nil || len(*resp.Kvs) == 0{
		//responsewriters.ErrorNegotiated(err,ls.NegotiatedSerializer,gv,w,req)
		responsewriters.InternalError(w,req,err)
		klog.Error(err)
		return
	}
	var unstrList unstructured.UnstructuredList
	for _,v := range *resp.Kvs {
		var unstrObj unstructured.Unstructured
		_,_,_= unstructured.UnstructuredJSONScheme.Decode([]byte(v.Value),nil,&unstrObj)
		unstrList.Items=append(unstrList.Items, unstrObj)
	}
	rv := strconv.FormatUint(resp.Revision,10)
	unstrList.SetResourceVersion(rv)
	unstrList.SetSelfLink(info.Path)
	unstrList.SetGroupVersionKind(gv.WithKind(util.UnsafeResourceToKind(info.Resource)+"List"))
	//responsewriters.WriteRawJSON(http.StatusOK,unstrList,w)
	responsewriters.WriteObjectNegotiated(ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions,gv,w,req,http.StatusOK,&unstrList)
	//buf := bytes.NewBuffer(nil)
	//_ = unstructured.UnstructuredJSONScheme.Encode(&unstrList,buf)
	////_ = unstrList.UnmarshalJSON(bytes)
	//writeRaw(http.StatusOK,buf.Bytes(),w)
	return
}
func(ls *LiteServer)processRead(w http.ResponseWriter,req *http.Request){
	ctx := req.Context()
	info, _  :=apirequest.RequestInfoFrom(ctx)
	key,err:= apiserverlite.KeyFuncReq(ctx,"")
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	resp, err := imitator.DefaultV2Client.Get(context.TODO(),key)
	gv := schema.GroupVersion{
		Group: info.APIGroup,
		Version: info.APIVersion,
	}
	if err !=nil || len(*resp.Kvs) == 0{
		responsewriters.ErrorNegotiated(err,ls.NegotiatedSerializer,gv,w,req)
		//responsewriters.InternalError(w,req,err)
		klog.Error(err)
		return
	}
	mdeiaType,_,err:= negotiation.NegotiateOutputMediaType(req,ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions)
	if err !=nil{
		responsewriters.ErrorNegotiated(err,ls.NegotiatedSerializer,gv,w,req)
		return
	}
	meta := (*resp.Kvs)[0]
	b := []byte(meta.Value)
	switch mdeiaType.Accepted.MediaType {
	case "application/json":
		writeRaw(http.StatusOK, b,w)
	default://"application/yaml" convert to yaml format
		var unstrObj unstructured.Unstructured
		_,_,err = unstructured.UnstructuredJSONScheme.Decode(b,nil,&unstrObj)
		if err!=nil{
			responsewriters.InternalError(w,req,err)
		}
		responsewriters.WriteObjectNegotiated(ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions,gv,w,req,http.StatusOK,&unstrObj)

	}
	return
}
func writeRaw(statusCode int,bytes []byte,w http.ResponseWriter){
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(bytes)
}
func(ls *LiteServer)processWatch(w http.ResponseWriter, req *http.Request){
	key,err:= apiserverlite.KeyFuncReq(req.Context(),"")
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	opts := storage.ListOptions{ResourceVersion:"0",Predicate: storage.Everything}
	watcher,err := ls.Storage.WatchList(req.Context(),key,opts)
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	//mdeiaType,_,err := negotiation.NegotiateOutputMediaType(req,ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions)
	serializerInfo,err := negotiation.NegotiateOutputMediaTypeStream(req,ls.NegotiatedSerializer,negotiation.DefaultEndpointRestrictions)
	if err !=nil{
		responsewriters.InternalError(w,req,err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		err := fmt.Errorf("unable to start watch - can't get http.Flusher: %#v", w)
		utilruntime.HandleError(err)
		responsewriters.InternalError(w,req,fmt.Errorf("failed to get http.Flusher"))
	}
	framer := serializerInfo.StreamSerializer.Framer.NewFrameWriter(w)
	streamSerializer := serializerInfo.StreamSerializer.Serializer
	encoder := ls.NegotiatedSerializer.EncoderForVersion(streamSerializer,nil)
	//useTextFraming := serializerInfo.EncodesAsText
	//mediaType := serializerInfo.MediaType
	e := streaming.NewEncoder(framer, encoder)
	mediaType := serializerInfo.MediaType
	if mediaType != runtime.ContentTypeJSON {
		mediaType += ";stream=watch"
	}

	// begin the stream
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	internalEvent := &metav1.InternalEvent{}
	outEvent := &metav1.WatchEvent{}
	ch := watcher.ResultChan()
	neverExitWatch := make(<-chan time.Time)
	for{
		select{
		case <-req.Context().Done():
			return
		case <-neverExitWatch:
			responsewriters.InternalError(w,req,fmt.Errorf("timeout to get event from watcher"))
			klog.Errorf("timeout for watching")
			return
		case event, ok := <-ch:
			if !ok {
				responsewriters.InternalError(w,req,fmt.Errorf("failed to get event from watcher"))
				return
			}
			*outEvent = metav1.WatchEvent{}

			// create the external type directly and encode it.  Clients will only recognize the serialization we provide.
			// The internal event is being reused, not reallocated so its just a few extra assignments to do it this way
			// and we get the benefit of using conversion functions which already have to stay in sync
			*internalEvent = metav1.InternalEvent(event)
			err := metav1.Convert_v1_InternalEvent_To_v1_WatchEvent(internalEvent, outEvent, nil)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to convert watch object: %v", err))
				// client disconnect.
				return
			}
			if err := e.Encode(outEvent); err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to encode watch object %T: %v (%#v)", outEvent, err, e))
				// client disconnect.
				return
			}
			if len(ch) == 0 {
				flusher.Flush()
			}

		}
	}
}
func(ls *LiteServer)processFail(w http.ResponseWriter,req *http.Request){

}
func BuildHandlerChain(handler http.Handler, ls *LiteServer) http.Handler {
	cfg := &server.Config{
		//TODO: support APIGroupPrefix /apis
		LegacyAPIGroupPrefixes: sets.NewString(server.DefaultLegacyAPIPrefix),
	}
	//handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, ls.LongRunningFunc, ls.RequestTimeout)
	handler = genericfilters.WithWaitGroup(handler, ls.LongRunningFunc, ls.HandlerChainWaitGroup)
	handler = genericapifilters.WithRequestInfo(handler,server.NewRequestInfoResolver(cfg))
	//handler = genericapifilters.WithWarningRecorder(handler)
	//handler = genericapifilters.WithCacheControl(handler)
	handler = genericfilters.WithPanicRecovery(handler)
	return handler
}

