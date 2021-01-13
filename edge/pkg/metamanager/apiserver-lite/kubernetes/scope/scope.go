package scope

import (
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/apiserver-lite/kubernetes/serializer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers"
)

func NewRequestScope (){
	requestScope :=handlers.RequestScope{
		Serializer:          serializer.NewNegotiatedSerializer(),
		ParameterCodec:      a.group.ParameterCodec,
			Creater:         a.group.Creater,
			Convertor:       a.group.Convertor,
			Defaulter:       a.group.Defaulter,
			Typer:           a.group.Typer,
			UnsafeConvertor: a.group.UnsafeConvertor,
			Authorizer:      a.group.Authorizer,

			EquivalentResourceMapper: a.group.EquivalentResourceRegistry,

		// TODO: Check for the interface on storage
			TableConvertor: tableProvider,

		// TODO: This seems wrong for cross-group subresources. It makes an assumption that a subresource and its parent are in the same group version. Revisit this.
			Resource:    a.group.GroupVersion.WithResource(resource),
			Subresource: subresource,
			Kind:        fqKindToRegister,

			HubGroupVersion: schema.GroupVersion{Group: fqKindToRegister.Group, Version: runtime.APIVersionInternal},

			MetaGroupVersion: metav1.SchemeGroupVersion,

			MaxRequestBodyBytes: a.group.MaxRequestBodyBytes,
	}

}
