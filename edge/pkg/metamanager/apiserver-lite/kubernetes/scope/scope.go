package scope

import (
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/kubernetes/serializer"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers"
	"k8s.io/client-go/kubernetes/scheme"
)

func NewRequestScope () *handlers.RequestScope{
	requestScope :=handlers.RequestScope{
		Namer: handlers.ContextBasedNaming{
			SelfLinker:         meta.NewAccessor(),
			ClusterScoped:      false,
			SelfLinkPathPrefix: "",
			SelfLinkPathSuffix: "",
		},

		Serializer:          serializer.NewNegotiatedSerializer(),
		ParameterCodec:      scheme.ParameterCodec,
		Creater:         nil,
		Convertor:       scheme.Scheme,
		Defaulter:       nil,
		Typer:           nil,
		UnsafeConvertor: nil,
		Authorizer:      nil,

		EquivalentResourceMapper: runtime.NewEquivalentResourceRegistry(),

		TableConvertor: nil,

		Resource:   schema.GroupVersionResource{},
		Subresource: "",
		Kind:        schema.GroupVersionKind{},

		HubGroupVersion: schema.GroupVersion{},

		MetaGroupVersion: metav1.SchemeGroupVersion,

		MaxRequestBodyBytes: int64(3 * 1024 * 1024),
	}
	return &requestScope
}
