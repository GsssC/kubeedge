package metaserver

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Test MetaServer", func() {
	Context("Test Access MetaServer at local", func() {
		BeforeEach(func() {
		})
		AfterEach(func() {
		})
		It("Test NotFound response", func() {
			var (
				coreAPIPrefix       = "api"
				coreAPIGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}
				prefix              = "apis"
				testGroupVersion    = schema.GroupVersion{Group: "test-group", Version: "test-version"}
			)
			type T struct {
				Method string
				Path   string
				Status int
			}
			cases := map[string]T{
				// Positive checks to make sure everything is wired correctly
				// Cluster-Scope API
				"List long prefix":	{"GET", "/" + coreAPIPrefix + "/", http.StatusNotFound},
				"List Core Cluster-Scope API":   {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes", http.StatusOK},
				"List Core Cluster-Scope API missing storage": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/foo", http.StatusNotFound},
				"Get Core Cluster-Scope API":   {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes/node-foo", http.StatusNotFound},
				"Get Core Cluster-Scope API with extra segment": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes/node-foo/baz", http.StatusNotFound},
				"Watch Core Cluster-Scope API": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes?watch=true", http.StatusOK}, //是ok吗?
				"Watch Core Cluster-Scope API with bad method": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/watch/nodes", http.StatusMethodNotAllowed},//?
				"Watch Core Cluster-Scope API missing storage": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/foo?watch=true", http.StatusNotFound},
				"Patch Core Cluster-Scope API": {"PATCH", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes", http.StatusMethodNotAllowed}, //?
				"Delete Core Cluster-Scope API list": {"DELETE", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes", http.StatusMethodNotAllowed}, //？
				"Delete Core Cluster-Scope API": {"DELETE", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes/node-foo", http.StatusNotFound},
				"Delete Core Cluster-Scope API with extra segment": {"DELETE", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes/node-foo/baz", http.StatusNotFound},
				"Replace Core Cluster-Scope API": {"PUT", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/nodes", http.StatusMethodNotAllowed},

				"List Cluster-Scope API":        {"GET", "/" + prefix + "/apiextensions.k8s.io/v1beta1/customresourcedefinitions", http.StatusOK},
				"Get Cluster-Scope API":        {"GET", "/" + prefix + "/apiextensions.k8s.io/v1beta1/customresourcedefinitions/crd-foo", http.StatusNotFound},
				"Watch Cluster-Scope API":        {"GET", "/" + prefix + "/apiextensions.k8s.io/v1beta1/customresourcedefinitions/crd-foo?watch=true", http.StatusNotFound},


				// Namespace-Scope API
				"List Core Namespace-Scope API": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods", http.StatusOK},
				"List Core Namespace-Scope API missing storage": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/foo", http.StatusNotFound},
				"Get Core Namespace-Scope API": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods/pod-foo", http.StatusNotFound},
				"Get Core Namespace-Scope API with extra segment": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods/pod-foo/baz", http.StatusNotFound},
				"Watch Core Namespace-Scope API": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods?watch=true", http.StatusOK},
				"Watch Core Namespace-Scope API missing storage": {"GET", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespces/ns/foo?watch=true", http.StatusNotFound},
				"Patch Core Namespace-Scope API": {"PATCH", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods", http.StatusMethodNotAllowed},
				"Delete Core Namespace-Scope API list": {"DELETE", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods", http.StatusMethodNotAllowed},
				"Delete Core Namespace-Scope API": {"DELETE", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods/pod-foo", http.StatusNotFound},
				"Delete Core Namespace-Scope API with extra segment": {"DELETE", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods/pod-foo/baz", http.StatusNotFound},
				"Replace Core Namespace-Scope API list": {"PUT", "/" + coreAPIPrefix + "/" + coreAPIGroupVersion.Version + "/namespaces/ns/pods", http.StatusMethodNotAllowed},

				"List Namespace-Scope API":      {"GET", "/" + prefix + "/apps/v1/namespaces/ns-foo/jobs", http.StatusOK},
				"Get Namespace-Scope API":      {"GET", "/" + prefix + "/apps/v1/namespaces/ns-foo/jobs/job-foo", http.StatusNotFound},
				"Watch Namespace-Scope API":      {"GET", "/" + prefix + "/apps/v1/namespaces/ns-foo/jobs/job-foo?watch=true", http.StatusNotFound},
			}
			client := http.Client{}
			url := "http://127.0.0.1:10550"
			for _, v := range cases {
				request, err := http.NewRequest(v.Method, url+v.Path, nil)
				Expect(err).Should(BeNil())
				response, err := client.Do(request)
				Expect(err).Should(BeNil())
				isEqual := v.Status == response.StatusCode
				Expect(isEqual).Should(BeTrue(), "Expected response status %v, Got %v", v.Status, response.Status)
			}
		})
	})
})
