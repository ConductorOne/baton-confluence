package connector

import (
	"context"
	"testing"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	"github.com/conductorone/baton-confluence/test"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"
)

func TestUsersList(t *testing.T) {
	ctx := context.Background()

	t.Run("should get users, using pagination, ignoring robots but including deactivated", func(t *testing.T) {
		server := test.FixturesServer()
		defer server.Close()

		confluenceClient, err := client.NewConfluenceClient(
			ctx,
			"username",
			"API Key",
			server.URL,
		)
		if err != nil {
			t.Fatal(err)
		}
		c := userBuilder(confluenceClient)

		resources := make([]*v2.Resource, 0)
		pToken := pagination.Token{Size: 2}
		for {
			nextResources, results, err := c.List(ctx, nil, resource.SyncOpAttrs{PageToken: pToken})
			resources = append(resources, nextResources...)

			require.Nil(t, err)
			if results == nil {
				break
			}
			test.AssertNoRatelimitAnnotations(t, results.Annotations)
			if results.NextPageToken == "" {
				break
			}
			pToken.Token = results.NextPageToken
		}

		require.NotNil(t, resources)
		// We expect there to be duplicates from users being in multiple groups
		// and then showing up in User Search. Now includes deactivated users.
		require.Len(t, resources, 7)
		require.NotEmpty(t, resources[0].Id)

		allIDs := mapset.NewSet[string]()
		for _, r := range resources {
			allIDs.Add(r.Id.Resource)
		}
		require.Equal(t, allIDs.Cardinality(), 3)
	})
}
