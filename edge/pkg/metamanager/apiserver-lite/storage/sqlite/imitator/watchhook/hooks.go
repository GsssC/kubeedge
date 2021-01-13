package watchhook

import (
	"fmt"
	"github.com/kubeedge/kubeedge/pkg/apiserverlite"
	"k8s.io/apimachinery/pkg/api/meta"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage/etcd3"
	"k8s.io/klog"
	"sync"
)

var(
	hooksLock sync.Mutex
	// map[hook.id]hook
	hooks  = make(map[string]*WatchHook)
)
func AddHook(hook *WatchHook) error {
	hooksLock.Lock()
	defer hooksLock.Unlock()
	if _, exists := hooks[hook.id]; exists {
		return fmt.Errorf("unable to add hook %v because it was already registered", hook.id)
	}
	hooks[hook.id] = hook
	return nil
}

func DeleteHook(id string) error {
	hooksLock.Lock()
	defer hooksLock.Unlock()
	if _, exists := hooks[id]; exists {
		delete(hooks, id)
		return nil
	}
	return fmt.Errorf("unable to delete %q because it was not registered", id)
}

// Trigger trigger the corresponding hook to serve watch based on the event passed in
func Trigger(e watch.Event){
	for _,hook := range hooks {
		compGVR,compNS,compName,compRev:= true, true,true,true
		key := apiserverlite.KeyFunc(e.Object)
		gvr,ns,name := apiserverlite.ParseKey(key)
		if !hook.GetGVR().Empty(){
			compGVR = hook.GetGVR() == gvr
		}
		if hook.GetNamespace() != ""{
			compNS = hook.GetNamespace() == ns
		}
		if hook.GetName() != ""{
			compName = hook.GetName() == name
		}
		if hook.GetResourceVersion() != 0{
			accessor,err := meta.Accessor(e.Object)
			if err !=nil{
				klog.Error(err)
				return
			}
			rev , err := etcd3.Versioner.ParseResourceVersion(accessor.GetResourceVersion())
			if err !=nil{
				klog.Error(err)
				return
			}
			compRev = hook.GetResourceVersion() < rev
		}
		if compGVR && compNS && compName && compRev{
			utilruntime.Must(hook.Do(e))
		}
	}
}
