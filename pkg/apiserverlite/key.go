package apiserverlite

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"
	"strings"
)

func KeyFunc(obj runtime.Object) string{
	key ,err := KeyFuncObj(obj)
	if err !=nil{
		klog.Errorf("failed to parse key from an obj:%v",err)
		return ""
	}
	return key
}
// KeyFuncObj generated key from obj.metadata.selflink
func KeyFuncObj(obj runtime.Object) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}
	key := accessor.GetSelfLink()
	return key,nil
}
// KeyFuncReq generate key from reqInfo.Path
func KeyFuncReq(ctx context.Context,_ string)(string,error){
	info , ok := apirequest.RequestInfoFrom(ctx)
	var key string
	// key example:
	// /0  /1   /2 /3         /4          /5   / 6
	// /api/v1/namespaces/{namespace}/pods/{name} GET
	// /api/v1/namespaces/{namespace}/pods/ LIST
	// /apis/extensions/v1beta1/namespaces/{namespace}/ingresses/{name} GET
	// /apis/storage.k8s.io/v1beta1/csidrivers/{name} GET cluster scope resource
	if ok && info.IsResourceRequest{
		key = "/"
		key += info.APIPrefix + "/"
		switch info.APIPrefix{
		case "api"://do nothing
		case "apis":
			if info.APIGroup==""{
				return "",fmt.Errorf("failed to get key from request info")
			}
			key += info.APIGroup + "/"
		default:
			return "",fmt.Errorf("failed to get key from request info")
		}
		key += info.APIVersion + "/"
		if info.Namespace != "" {
			key += "namespaces" + "/"
			key += info.Namespace + "/"
		}
		key += info.Resource +"/"
		if info.Name != ""{
			key += info.Name
		}
	}else{
		return "",fmt.Errorf("no request info in context")
	}
	klog.Infof("[apiserver-lite]get a req, key:%v",key)
	return key, nil

}
func KeyRootFunc(ctx context.Context)string{
	key ,err := KeyFuncReq(ctx,"")
	if err !=nil {
		panic("fail to get list key!")
	}
	return key
}
// ParseKey parse key to group/version/resource, namespace, name
// Now key is equal to selflink like below:
// 0/1  /2 /3         /4      /5   /6
//  /api/v1/namespaces/default/pods/pod-foo
// 0/1   /2  /3 /4
//  /apis/app/v1/deployments
// 0/1  /2 /3
//  /api/v1/endpoints
// abnormal:
// 0/1
//  /apis/v1/pods
func ParseKey(selflink string)(gvr schema.GroupVersionResource,namespace string,name string){
	sl := selflink
	slices := strings.Split(sl,"/")
	if len(slices)<4{
		return
	}
	var apiVersion string
	var versionIndex int
	var namespaceIndex int
	var resourceIndex int
	//TODO: to be more safer
	switch {
	// /api/v1/pods
	// /apis/group/version/pods
	case slices[1] == "api":
		versionIndex = 2
		apiVersion = slices[versionIndex]
	case slices[1] =="apis":
		if len(slices)==4{
			return
		}
		groupIndex := 2
		versionIndex = 3
		apiVersion = slices[groupIndex]+"/"+slices[versionIndex]
	default:
		return
	}
	if strings.Contains(sl,"/namespaces/"){
		resourceIndex = versionIndex+3
		namespaceIndex = versionIndex+2
	}else{
		resourceIndex = versionIndex+1
		namespaceIndex = 0
	}
	if namespaceIndex >=len(slices){
		namespaceIndex = 0
	}
	if resourceIndex >=len(slices){
		resourceIndex = 0
	}
	nameIndex := resourceIndex+1
	if nameIndex >= len(slices){
		nameIndex = 0
	}
	namespace = slices[namespaceIndex]
	resource := slices[resourceIndex]
	name = slices[nameIndex]
	gv , err := schema.ParseGroupVersion(apiVersion)
	if err!=nil{
		klog.Error(err)
	}
	gvr = gv.WithResource(resource)
	return gvr,namespace,name
}
