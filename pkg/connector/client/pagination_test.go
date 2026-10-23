package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/stretchr/testify/require"
)

// newPaginationTestClient points a client at a stub Confluence that always replies with
// body. Each test gets its own server, so the uhttp response cache cannot carry a body
// between them.
func newPaginationTestClient(t *testing.T, body string) *ConfluenceClient {
	t.Helper()

	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set(uhttp.ContentType, "application/json")
			writer.WriteHeader(http.StatusOK)
			_, err := writer.Write([]byte(body))
			if err != nil {
				return
			}
		}),
	)
	t.Cleanup(server.Close)

	client, err := NewConfluenceClient(context.Background(), "user", "api-key", server.URL)
	require.NoError(t, err)

	return client
}

// Confluence keeps _links on the last page and simply omits next. That is the end of the
// sync, not a failure.
func TestGetGroupsLastPage(t *testing.T) {
	client := newPaginationTestClient(t, `{"results":[{"id":"g1","name":"devs"}],"start":0,"limit":2,"size":1,"_links":{"base":"https://example.atlassian.net/wiki"}}`)

	groups, nextToken, _, err := client.GetGroups(context.Background(), "", 2)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Empty(t, nextToken, "the last page should not advance the cursor")
}

// The case the opt-in exists for. A 200 carrying results but no _links yields an empty
// cursor, which is indistinguishable from the last page.
func TestGetGroupsMissingLinks(t *testing.T) {
	client := newPaginationTestClient(t, `{"results":[{"id":"g1","name":"devs"}],"start":0,"limit":2,"size":1}`)

	_, _, _, err := client.GetGroups(context.Background(), "", 2)
	require.Error(t, err, "a page without _links should not look like the last page")
	require.ErrorIs(t, err, uhttp.ErrMissingPaginationData)
}

func TestGetSpacesMissingLinks(t *testing.T) {
	client := newPaginationTestClient(t, `{"results":[{"id":"s1","key":"SP"}]}`)

	_, _, _, err := client.GetSpaces(context.Background(), 2, "")
	require.Error(t, err)
	require.ErrorIs(t, err, uhttp.ErrMissingPaginationData)
}

func TestGetSpaceRolesMissingLinks(t *testing.T) {
	client := newPaginationTestClient(t, `{"results":[{"id":"r1"}]}`)

	_, _, _, err := client.GetSpaceRoles(context.Background(), "s1", "", 2)
	require.Error(t, err)
	require.ErrorIs(t, err, uhttp.ErrMissingPaginationData)
}

// The space-roles and role-assignments endpoints return _links as an empty object. That
// is still the block arriving, so it must pass.
func TestGetSpaceRolesEmptyLinksObject(t *testing.T) {
	client := newPaginationTestClient(t, `{"results":[{"id":"r1"}],"_links":{}}`)

	roles, nextToken, _, err := client.GetSpaceRoles(context.Background(), "s1", "", 2)
	require.NoError(t, err, "an empty _links object is present, just without a next link")
	require.Len(t, roles, 1)
	require.Empty(t, nextToken)
}

// GetSpaceById is a single-resource read whose body carries no _links at all, so it must
// not start demanding one even though it shares the same request path.
func TestGetSpaceByIdDoesNotRequirePagination(t *testing.T) {
	client := newPaginationTestClient(t, `{"id":"s1","key":"SP","name":"Space","type":"global","status":"current"}`)

	space, _, err := client.GetSpaceById(context.Background(), "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", space.Id)
}
