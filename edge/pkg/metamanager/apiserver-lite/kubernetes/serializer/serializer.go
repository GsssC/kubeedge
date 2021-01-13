package serializer

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructuredscheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

// NewUnstructuredNegotiatedSerializer returns a simple, negotiated serializer
func NewNegotiatedSerializer() runtime.NegotiatedSerializer {
	return WithoutConversionCodecFactory{
		typer:   unstructuredscheme.NewUnstructuredObjectTyper(),
		creator: unstructuredscheme.NewUnstructuredCreator(),
	}
}
type WithoutConversionCodecFactory struct {
	creator runtime.ObjectCreater
	typer   runtime.ObjectTyper
}
func (f WithoutConversionCodecFactory) SupportedMediaTypes() []runtime.SerializerInfo {
	return []runtime.SerializerInfo{
		{
			MediaType:        "application/json",
			MediaTypeType:    "application",
			MediaTypeSubType: "json",
			EncodesAsText:    true,
			Serializer:       json.NewSerializer(json.DefaultMetaFactory, f.creator, f.typer, false),
			PrettySerializer: json.NewSerializer(json.DefaultMetaFactory, f.creator, f.typer, true),
			StreamSerializer: &runtime.StreamSerializerInfo{
				EncodesAsText: true,
				Serializer:    json.NewSerializer(json.DefaultMetaFactory, f.creator, f.typer, false),
				Framer:        json.Framer,
			},
		},
		{
			MediaType:        "application/yaml",
			MediaTypeType:    "application",
			MediaTypeSubType: "yaml",
			EncodesAsText:    true,
			Serializer:       json.NewYAMLSerializer(json.DefaultMetaFactory, f.creator, f.typer),
		},
	}
}

// EncoderForVersion do nothing but return encoder
func (f WithoutConversionCodecFactory) EncoderForVersion(serializer runtime.Encoder, _ runtime.GroupVersioner) runtime.Encoder {
	return serializer
}

// DecoderToVersion do nothing but return decoder
func (f WithoutConversionCodecFactory) DecoderToVersion(serializer runtime.Decoder, _ runtime.GroupVersioner) runtime.Decoder {
	return serializer
}
