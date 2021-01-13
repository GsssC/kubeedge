package util

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
)

// TODO: selflink will be removed at k8s v1.21, choose another way to recover MetaType information
// MetaType is generally consisted of apiversion, kind like:
// {
// apiVersion: apps/v1
// kind: Deployments
// }
// And we add these information here from selflink, and it is like below:
// 1.selflink of an obj
// 0/1  /2 /3         /4      /5   /6
//  /api/v1/namespaces/default/pods/pod-foo
// 2.selflink of a List obj:
// 0/1   /2  /3 /4
//  /apis/apps/v1/deployments
func SetMetaType(obj runtime.Object)error{
	accessor, err:= meta.Accessor(obj)
	if err !=nil{
		return err
	}
	sl := accessor.GetSelfLink()
	slices := strings.Split(sl,"/")
	// TODO: to be more safe
	var apiVersion string
	var resource string
	//var gvr schema.GroupVersionResource
	var gvk schema.GroupVersionKind
	switch {
	case slices[1] == "api":
		versionIndex := 2
		apiVersion = slices[versionIndex]
		if strings.Contains(sl,"/namespaces/"){
			resource = slices[versionIndex+3]
		}else{
			resource = slices[versionIndex+1]
		}

	case slices[1] =="apis":
		groupIndex := 2
		versionIndex := 3
		apiVersion = slices[groupIndex]+"/"+slices[versionIndex]
		if strings.Contains(sl,"/namespaces/"){
			resource = slices[versionIndex+3]
		}else{
			resource = slices[versionIndex+1]
		}
	}
	gvk = schema.FromAPIVersionAndKind(apiVersion, UnsafeResourceToKind(resource))
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	return nil
}
// Sometimes, we need guess kind according to resource:
// 1. In most cases, is like pods to Pod,
// 2. In some unusual cases, requires special treatment like endpoints to Endpoints
func UnsafeResourceToKind(r string) string{
	if len(r)==0{
		return r
	}
	unusualResourceToKind := map[string]string{
		"endpoints":"Endpoints",
	}
	if v, isUnusual := unusualResourceToKind[r]; isUnusual {
		return v
	}
	k := strings.Title(r)
	switch{
	case strings.HasSuffix(k,"ies"):
		return strings.TrimSuffix(k,"ies") + "y"
	case strings.HasSuffix(k,"es"):
		return strings.TrimSuffix(k,"es")
	case strings.HasSuffix(k,"s"):
		return strings.TrimSuffix(k,"s")
	}
	return k
}

