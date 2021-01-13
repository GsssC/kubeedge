package apiserverlite

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

// TestSaveMeta is function to initialize all global variable and test SaveMeta
func TestParseKey(t *testing.T) {
	type result struct{
		gvr schema.GroupVersionResource
		namespace string
		name string
	}
	cases := []struct {
		key string
		stdResult result
	}{
		{
			// Success Case
			key:"/api/v1/namespaces/default/pod/pod-foo",
			stdResult: result {
				gvr:schema.GroupVersionResource{
					Group: "",
					Version: "v1",
					Resource: "pod",
				},
				namespace:"default",
				name: "pod-foo",
			},
		},
		{
			key:"/api/v1/endpoints",
			stdResult: result{
				gvr:schema.GroupVersionResource{
					Group: "",
					Version: "v1",
					Resource: "endpoints",
				},
				namespace: "",
				name: "",
			},
		},
		{//fail test
			key:"/apis/v1/endpoints",
			stdResult: result{
				gvr:schema.GroupVersionResource{
					Group: "",
					Version: "",
					Resource: "",
				},
				namespace: "",
				name: "",
			},
		},
		{//fail test
			key:"/apiss/v1/endpoints",
			stdResult: result{
				gvr:schema.GroupVersionResource{
					Group: "",
					Version: "",
					Resource: "",
				},
				namespace: "",
				name: "",
			},
		},
		{//fail test
			key:"/apiss/apps/v1/deployments",
			stdResult: result{
				gvr:schema.GroupVersionResource{
					Group: "",
					Version: "",
					Resource: "",
				},
				namespace: "",
				name: "",
			},
		},
		{//fail test
			key:"/api/v1/endpoints/",
			stdResult: result{
				gvr:schema.GroupVersionResource{
					Group: "",
					Version: "v1",
					Resource: "endpoints",
				},
				namespace: "",
				name: "",
			},
		},
		{//fail test
			key:"/api/v1/namespaces/default/endpoints/",
			stdResult: result{
				gvr:schema.GroupVersionResource{
					Group: "",
					Version: "v1",
					Resource: "endpoints",
				},
				namespace: "default",
				name: "",
			},
		},
	}

	// run the test cases
	for _, test := range cases {
		t.Run("parseKey", func(t *testing.T) {
			gvr,ns,name := ParseKey(test.key)
			parseResult := result{gvr,ns,name}
			if test.stdResult != parseResult{
				t.Errorf("ParseKey Case failed, key:%v,wanted result:%+v,acctual result:%+v",test.key,test.stdResult,parseResult)
			}
		})
	}
}
