package test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
)

func AssertNoRatelimitAnnotations(
	t *testing.T,
	actualAnnotations annotations.Annotations,
) {
	if actualAnnotations != nil && len(actualAnnotations) == 0 {
		return
	}

	for _, annotation := range actualAnnotations {
		var ratelimitDescription v2.RateLimitDescription
		err := annotation.UnmarshalTo(&ratelimitDescription)
		if err != nil {
			continue
		}
		if slices.Contains(
			[]v2.RateLimitDescription_Status{
				v2.RateLimitDescription_STATUS_ERROR,
				v2.RateLimitDescription_STATUS_OVERLIMIT,
			},
			ratelimitDescription.Status,
		) {
			t.Fatal("request was ratelimited, expected not to be ratelimited")
		}
	}
}

func FixturesServer() *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set(uhttp.ContentType, "application/json")
				writer.WriteHeader(http.StatusOK)
				var filename string
				routeUrl := request.URL.String()
				switch {
				case strings.Contains(routeUrl, client.GroupsListUrlPath) && strings.Contains(routeUrl, "123"):
					filename = "../../test/fixtures/users0.json"
				case strings.Contains(routeUrl, client.GroupsListUrlPath) && strings.Contains(routeUrl, "456"):
					filename = "../../test/fixtures/users1.json"
				case strings.Contains(routeUrl, client.GroupsListUrlPath):
					filename = "../../test/fixtures/groups.json"
				case strings.Contains(routeUrl, client.SpacesListUrlPath) && strings.Contains(routeUrl, "permissions"):
					filename = "../../test/fixtures/permissions0.json"
				case strings.Contains(routeUrl, client.SpacesListUrlPath) && strings.Contains(routeUrl, "cursor"):
					filename = "../../test/fixtures/spaces1.json"
				case strings.Contains(routeUrl, client.SpacesListUrlPath) && !strings.Contains(routeUrl, "cursor"):
					filename = "../../test/fixtures/spaces0.json"
				default:
					// This should never happen in tests.
					return
				}
				data, _ := os.ReadFile(filename)
				_, err := writer.Write(data)
				if err != nil {
					return
				}
			},
		),
	)
}