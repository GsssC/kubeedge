package storagebackend

/*

import (
	"github.com/astaxie/beego/orm"
	"github.com/kubeedge/beehive/pkg/core/model"
	"github.com/kubeedge/kubeedge/edge/pkg/common/dbm"
	v2 "github.com/kubeedge/kubeedge/edge/pkg/metamanager/dao/v2"
	"github.com/kubeedge/kubeedge/pkg/apiserverlite"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"net/http"
	"strings"
	"time"
)

func CacheWatch(key, msg model.Message){
	gvr,ns,name:=ParseKey(key)
	time := time.Now().String()
	wc := WatchCache{
		Key:key+"/"+time,
		GroupVersionResource:buildGVRFrom(gvr),
		Namespace: ns,
		Name: name,
		ResourceVersion:0,
	}
}

func toKind(Resource string) string {
	switch{
	case strings.HasSuffix(Resource,"ies"):
		return strings.TrimSuffix(Resource,"ies") + "y"
	case strings.HasSuffix(Resource,"es"):
		return strings.TrimSuffix(Resource,"es")
	case strings.HasSuffix(Resource,"s"):
		return strings.TrimSuffix(Resource,"s")
	}
	return ""
}


func (c *client)xInsertOrUpdate(ctx context.Context, key string,obj runtime.Object,rv uint64) error {
	gvr, ns, name := apiserverlite.ParseKey(key)
	data ,err := json.Marshal(obj)
	utilruntime.Must(err)
	metaRecord := v2.MetaV2{
		Key:                  key,
		GroupVersionResource: gvr.String(),
		Namespace:            ns,
		Name:                 name,
		ResourceVersion:      rv,
		Value:                string(data),
	}
	c.lock.Lock()
	_, err = dbm.DBAccess.InsertOrUpdate(metaRecord)
	utilruntime.Must(err)
	if rv > c.GetRevision(){
		c.SetRevision(rv)
	}
	c.lock.Unlock()
	return err
}
func xKeyFuncObj(obj runtime.Object) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}
	var key string

	_ , singular := meta.UnsafeGuessKindToResource(obj.GetObjectKind().GroupVersionKind())
	if singular.Group == ""{
		key += "/api"+"/"
		key += v2.GroupCore + "/"
	}else{
		key += "/apis"+"/"
		key += singular.Group + "/"
	}
	key += singular.Version + "/"
	key += "namespaces" + "/"
	if accessor.GetNamespace() != ""{
		key += accessor.GetNamespace() + "/"
	} else{
		key += v2.NullNamespace + "/"
	}
	key += singular.Resource + "/"
	if accessor.GetName() != ""{
		key += accessor.GetName()
	}else{
		key += v2.NullName
	}
	return key,nil
}

func QueryByGVR(gvr schema.GroupVersionResource)(){

}
// WatchCache cache events that metamanager received
type WatchCache struct{
	// Key
	Key string `orm:"column(key); size(256); pk"`
	// GroupVersionResource is like "apps/v1, Resource=depoyments" or "/v1, Resource=pods"
	GroupVersionResource   string `orm:"column(groupversionresource); size(256);"`
	// Namespace is the namespace of an api object
	Namespace  string `orm:"column(namespace); size(256)"`
	// Name is the name of api object, metadata.name
	Name   string `orm:"column(name); size(256)"`
	// ResourceVersion is the resource version of the obj stored in value
	ResourceVersion uint64 `orm:"column(resourceversion); size(256); pk"`
	Message         string `orm:"column(message); null; type(text)"`
}
type WatchChan <-chan watch.Event
func QueryWatchCache(condition orm.Condition)(*[]string, error){
	events := new([]WatchCache)
	_, err := dbm.DBAccess.QueryTable(v2.WatchCacheTableName).SetCond(&condition).All(events)
	if err !=nil{
		return nil,err
	}
	var result []string
	for _,v := range *events{
		result = append(result,v.Message)
	}
	return &result,nil

}
func xBuildGVRFrom(gvkr interface{}) string {
	return buildGVRFrom(gvkr)
}
// List a slice of raw data by Group Version Resource Namespace Name
func RawByGVRNN(gvr schema.GroupVersionResource,namespace string,name string)(*[]string,error){
	objs := new([]v2.MetaV2)
	var err error
	switch namespace{
	case v2.NullNamespace:
		switch name {
		case v2.NullName:
			_, err = dbm.DBAccess.QueryTable(v2.NewMetaTableName).Filter(v2.GVR, gvr.String()).All(objs)
		default:
			_, err = dbm.DBAccess.QueryTable(v2.NewMetaTableName).Filter(v2.GVR, gvr.String()).Filter(v2.NAME,name).All(objs)
		}
	default:
		switch name {
		case v2.NullName:
			_, err = dbm.DBAccess.QueryTable(v2.NewMetaTableName).Filter(v2.GVR, gvr.String()).Filter(v2.NS,namespace).All(objs)
		default:
			_, err = dbm.DBAccess.QueryTable(v2.NewMetaTableName).Filter(v2.GVR, gvr.String()).Filter(v2.NS,namespace).Filter(v2.NAME,name).All(objs)
		}
	}
	if err != nil {
		return nil, err
	}
	var result []string
	for _, v := range *objs {
		result = append(result, v.Value)
	}
	return &result, nil
}

// Get one raw data by Group Version Kind Namespace name
func RawGetByGVRNN(gvr schema.GroupVersionResource,namespace string,name string)(string,error) {
	results, err := RawByGVRNN(gvr, namespace, name)
	if err != nil {
		return "", err
	}
	if results == nil {
		return "", nil
	}
	switch {
	case len(*results) == 1:
		return (*results)[0], nil
	default:
		return "", nil
	}
}
func RawMetaByKey(key string)(*[]MetaV2,error){
	objs := new([]MetaV2)
	var err error
	_, err = dbm.DBAccess.QueryTable(NewMetaTableName).Filter(KEY,key).All(objs)
}

func xParseKey(key string)(gvr schema.GroupVersionResource,namespace string, name string) {
	//0/1  /2   /3 /4         /5          /6   /7
	// /api/core/v1/namespaces/{namespace}/pods/{name} GET
	klog.Infof("parse key: %v",key)
	items := strings.Split(key,"/")
	gvr = schema.GroupVersionResource{
		Group: items[2],
		Version: items[3],
		Resource: items[6],
	}
	namespace = items[5]
	name = items[7]
	return
}

//
func CompareGVRK(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) bool{
	gvr1 := buildGVRFrom(gvr)
	gvr2 := buildGVRFrom(gvk)
	return gvr1 == gvr2
}
// build key name from GroupVersionResource as a, specially resource is singular ratherthan
func buildGVRFrom(gvkr interface{}) string {
	switch i := gvkr.(type){
	case schema.GroupVersionResource:
		switch {
		case i.Group == "":
			return strings.Join([]string{"core",i.Version, UnsafeResourceSingularizer(i.Resource)},"/")
		default :
			return strings.Join([]string{i.Group,i.Version, UnsafeResourceSingularizer(i.Resource)},"/")
		}
	case schema.GroupVersionKind:
		_,singular:=meta.UnsafeGuessKindToResource(i)
		return buildGVRFrom(singular)
	default:
		klog.Warningf("do not support build GVK from: %+v",gvkr)
		return ""
	}
}
// It converts a resource name from plural to singular (e.g., from pods to pod)
func UnsafeResourceSingularizer(plural string) string {
	switch{
	case strings.HasSuffix(plural,"ies"):
		return strings.TrimSuffix(plural,"ies") + "y"
	case strings.HasSuffix(plural,"es"):
		return strings.TrimSuffix(plural,"es")
	case strings.HasSuffix(plural,"s"):
		return strings.TrimSuffix(plural,"s")
	default:
		return plural
	}
}

func DefaultBuildHandlerChain(apiHandler http.Handler, c *server.Config) http.Handler {
	handler := apiHandler
	//handler = genericapifilters.WithAuthorization(apiHandler, c.Authorization.Authorizer, c.Serializer)
	// TODO: temporarily not add flow control
	//if c.FlowControl != nil {
	//	handler = genericfilters.WithPriorityAndFairness(handler, c.LongRunningFunc, c.FlowControl)
	//} else {
	//	handler = genericfilters.WithMaxInFlightLimit(handler, c.MaxRequestsInFlight, c.MaxMutatingRequestsInFlight, c.LongRunningFunc)
	//}
	handler = genericapifilters.WithImpersonation(handler, c.Authorization.Authorizer, c.Serializer)
	// TODO: temporarily not add audit
	//handler = genericapifilters.WithAudit(handler, c.AuditBackend, c.AuditPolicyChecker, c.LongRunningFunc)
	// TODO: temporarily not add safety related
	//failedHandler := genericapifilters.Unauthorized(c.Serializer)
	//failedHandler = genericapifilters.WithFailedAuthenticationAudit(failedHandler, c.AuditBackend, c.AuditPolicyChecker)
	//handler = genericapifilters.WithAuthentication(handler, c.Authentication.Authenticator, failedHandler, c.Authentication.APIAudiences)
	//handler = genericfilters.WithCORS(handler, c.CorsAllowedOriginList, nil, nil, nil, "true")
	handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, c.LongRunningFunc, c.RequestTimeout)
	handler = genericfilters.WithWaitGroup(handler, c.LongRunningFunc, c.HandlerChainWaitGroup)
	handler = genericapifilters.WithRequestInfo(handler, c.RequestInfoResolver)
	//if c.SecureServing != nil && !c.SecureServing.DisableHTTP2 && c.GoawayChance > 0 {
	//	handler = genericfilters.WithProbabilisticGoaway(handler, c.GoawayChance)
	//}
	//handler = genericapifilters.WithAuditAnnotations(handler, c.AuditBackend, c.AuditPolicyChecker)
	handler = genericapifilters.WithWarningRecorder(handler)
	handler = genericapifilters.WithCacheControl(handler)
	handler = genericfilters.WithPanicRecovery(handler)
	return handler
}

 */
