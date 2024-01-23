package definitions

import (
	"fmt"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	wschemas "github.com/rancher/wrangler/v2/pkg/schemas"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/proto"
)

func TestRefresh(t *testing.T) {
	defaultDocument, err := openapi_v2.ParseDocument([]byte(openapi_raw))
	require.NoError(t, err)
	defaultModels, err := proto.NewOpenAPIData(defaultDocument)
	require.NoError(t, err)
	defaultSchemaToModel := map[string]string{
		"management.cattle.io.globalrole": "io.cattle.management.v2.GlobalRole",
	}
	tests := []struct {
		name                     string
		openapiError             error
		serverGroupsResourcesErr error
		useBadOpenApiDoc         bool
		unparseableGV            bool
		wantModels               *proto.Models
		wantSchemaToModel        map[string]string
		wantError                bool
	}{
		{
			name:              "success",
			wantModels:        &defaultModels,
			wantSchemaToModel: defaultSchemaToModel,
		},
		{
			name:         "error - openapi doc unavailable",
			openapiError: fmt.Errorf("server unavailable"),
			wantError:    true,
		},
		{
			name:             "error - unable to parse openapi doc",
			useBadOpenApiDoc: true,
			wantError:        true,
		},
		{
			name:                     "error - unable to retrieve groups and resources",
			serverGroupsResourcesErr: fmt.Errorf("server not available"),
			wantModels:               &defaultModels,
			wantError:                true,
		},
		{
			name: "error - unable to retrieve all groups and resources",
			serverGroupsResourcesErr: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{
						Group:   "other.cattle.io",
						Version: "v1",
					}: fmt.Errorf("some group error"),
				},
			},
			wantModels:        &defaultModels,
			wantSchemaToModel: defaultSchemaToModel,
			wantError:         true,
		},
		{
			name:              "error - unparesable gv",
			unparseableGV:     true,
			wantModels:        &defaultModels,
			wantSchemaToModel: defaultSchemaToModel,
			wantError:         true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, err := buildDefaultDiscovery()
			client.DocumentErr = test.openapiError
			client.GroupResourcesErr = test.serverGroupsResourcesErr
			if test.useBadOpenApiDoc {
				schema := client.Document.Definitions.AdditionalProperties[0]
				schema.Value.Type = &openapi_v2.TypeItem{
					Value: []string{"multiple", "entries"},
				}
			}
			if test.unparseableGV {
				client.Resources = append(client.Resources, &metav1.APIResourceList{
					GroupVersion: "not/parse/able",
				})
			}
			require.Nil(t, err)
			handler := SchemaDefinitionHandler{
				client: client,
			}
			err = handler.Refresh()
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantModels, handler.models)
			require.Equal(t, test.wantSchemaToModel, handler.schemaToModel)
		})

	}
}

func Test_byID(t *testing.T) {
	defaultDocument, err := openapi_v2.ParseDocument([]byte(openapi_raw))
	require.NoError(t, err)
	defaultModels, err := proto.NewOpenAPIData(defaultDocument)
	require.NoError(t, err)
	defaultSchemaToModel := map[string]string{
		"management.cattle.io.globalrole": "io.cattle.management.v2.GlobalRole",
	}
	schemas := types.EmptyAPISchemas()
	addBaseSchema := func(names ...string) {
		for _, name := range names {
			schemas.MustAddSchema(types.APISchema{
				Schema: &wschemas.Schema{
					ID:                name,
					CollectionMethods: []string{"get"},
					ResourceMethods:   []string{"get"},
				},
			})
		}
	}

	intPtr := func(input int) *int {
		return &input
	}

	addBaseSchema("management.cattle.io.globalrole", "management.cattle.io.missingfrommodel", "management.cattle.io.notakind")

	tests := []struct {
		name          string
		schemaName    string
		models        *proto.Models
		schemaToModel map[string]string
		wantObject    *types.APIObject
		wantError     bool
		wantErrorCode *int
	}{
		{
			name:          "global role definition",
			schemaName:    "management.cattle.io.globalrole",
			models:        &defaultModels,
			schemaToModel: defaultSchemaToModel,
			wantObject: &types.APIObject{
				ID:   "management.cattle.io.globalrole",
				Type: "schemaDefinition",
				Object: schemaDefinition{
					DefinitionType: "io.cattle.management.v2.GlobalRole",
					Definitions: map[string]definition{
						"io.cattle.management.v2.GlobalRole": {
							ResourceFields: map[string]definitionField{
								"apiVersion": {
									Type:        "string",
									Description: "The APIVersion of this resource",
								},
								"kind": {
									Type:        "string",
									Description: "The kind",
								},
								"metadata": {
									Type:        "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
									Description: "The metadata",
								},
								"spec": {
									Type: "io.cattle.management.v2.GlobalRole.spec", Description: "The spec for the project",
								},
							},
							Type:        "io.cattle.management.v2.GlobalRole",
							Description: "A Global Role V2 provides Global Permissions in Rancher",
						},
						"io.cattle.management.v2.GlobalRole.spec": {
							ResourceFields: map[string]definitionField{
								"clusterName": {
									Type:        "string",
									Description: "The name of the cluster",
									Required:    true,
								},
								"displayName": {
									Type:        "string",
									Description: "The UI readable name",
									Required:    true,
								},
								"newField": {
									Type:        "string",
									Description: "A new field not present in v1",
								},
								"notRequired": {
									Type:        "boolean",
									Description: "Some field that isn't required",
								},
							},
							Type:        "io.cattle.management.v2.GlobalRole.spec",
							Description: "The spec for the project",
						},
						"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta": {
							ResourceFields: map[string]definitionField{
								"annotations": {
									Type:        "map",
									SubType:     "string",
									Description: "annotations of the resource",
								},
								"name": {
									Type:        "string",
									SubType:     "",
									Description: "name of the resource",
								},
							},
							Type:        "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
							Description: "Object Metadata",
						},
					},
				},
			},
		},
		{
			name:          "missing definition",
			schemaName:    "management.cattle.io.cluster",
			models:        &defaultModels,
			schemaToModel: defaultSchemaToModel,
			wantError:     true,
			wantErrorCode: intPtr(404),
		},
		{
			name:          "not refreshed",
			schemaName:    "management.cattle.io.globalrole",
			wantError:     true,
			wantErrorCode: intPtr(503),
		},
		{
			name:          "has schema, missing from model",
			schemaName:    "management.cattle.io.missingfrommodel",
			models:        &defaultModels,
			schemaToModel: defaultSchemaToModel,
			wantError:     true,
			wantErrorCode: intPtr(503),
		},
		{
			name:       "has schema, model is not a kind",
			schemaName: "management.cattle.io.notakind",
			models:     &defaultModels,
			schemaToModel: map[string]string{
				"management.cattle.io.notakind": "io.management.cattle.NotAKind",
			},
			wantError:     true,
			wantErrorCode: intPtr(500),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := SchemaDefinitionHandler{
				models:        test.models,
				schemaToModel: test.schemaToModel,
			}
			request := types.APIRequest{
				Schemas: schemas,
				Name:    test.schemaName,
			}
			response, err := handler.byIDHandler(&request)
			if test.wantError {
				require.Error(t, err)
				if test.wantErrorCode != nil {
					require.True(t, apierror.IsAPIError(err))
					apiErr, _ := err.(*apierror.APIError)
					require.Equal(t, *test.wantErrorCode, apiErr.Code.Status)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, *test.wantObject, response)
			}
		})
	}
}

func buildDefaultDiscovery() (*fakeDiscovery, error) {
	document, err := openapi_v2.ParseDocument([]byte(openapi_raw))
	if err != nil {
		return nil, fmt.Errorf("unable to parse openapi document %w", err)
	}
	groups := []*metav1.APIGroup{
		{
			Name: "management.cattle.io",
			PreferredVersion: metav1.GroupVersionForDiscovery{
				Version: "v2",
			},
		},
	}
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: schema.GroupVersion{
				Group:   "management.cattle.io",
				Version: "v2",
			}.String(),
			APIResources: []metav1.APIResource{
				{
					Group:   "management.cattle.io",
					Kind:    "GlobalRole",
					Version: "v2",
				},
			},
		},
		{
			GroupVersion: schema.GroupVersion{
				Group:   "management.cattle.io",
				Version: "v1",
			}.String(),
			APIResources: []metav1.APIResource{
				{
					Group:   "management.cattle.io",
					Kind:    "GlobalRole",
					Version: "v2",
				},
			},
		},
		nil,
	}
	return &fakeDiscovery{
		Groups:    groups,
		Resources: resources,
		Document:  document,
	}, nil
}

type fakeDiscovery struct {
	Groups            []*metav1.APIGroup
	Resources         []*metav1.APIResourceList
	Document          *openapi_v2.Document
	GroupResourcesErr error
	DocumentErr       error
}

// ServerGroupsAndResources is the only method we actually need for the test - just returns what is on the struct
func (f *fakeDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return f.Groups, f.Resources, f.GroupResourcesErr
}

// The rest of these methods are just here to conform to discovery.DiscoveryInterface
func (f *fakeDiscovery) RESTClient() restclient.Interface            { return nil }
func (f *fakeDiscovery) ServerGroups() (*metav1.APIGroupList, error) { return nil, nil }
func (f *fakeDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	return nil, nil
}
func (f *fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (f *fakeDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (f *fakeDiscovery) ServerVersion() (*version.Info, error) { return nil, nil }
func (f *fakeDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return f.Document, f.DocumentErr
}
func (f *fakeDiscovery) OpenAPIV3() openapi.Client                { return nil }
func (f *fakeDiscovery) WithLegacy() discovery.DiscoveryInterface { return f }
