package connector

import (
	"context"
	"testing"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	"github.com/conductorone/baton-confluence/test"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"
)

func TestUsersList(t *testing.T) {
	ctx := context.Background()

	t.Run("should get users, using pagination, ignoring robots & deactivated", func(t *testing.T) {
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
		bag := &pagination.Bag{}
		for {
			pToken := pagination.Token{Size: 2}
			state := bag.Current()
			if state != nil {
				token, _ := bag.Marshal()
				pToken.Token = token
			}

			nextResources, nextToken, listAnnotations, err := c.List(ctx, nil, &pToken)
			resources = append(resources, nextResources...)

			require.Nil(t, err)
			test.AssertNoRatelimitAnnotations(t, listAnnotations)
			if nextToken == "" {
				break
			}

			err = bag.Unmarshal(nextToken)
			if err != nil {
				t.Error(err)
			}
		}

		require.NotNil(t, resources)
		// We expect there to be duplicates from users being in multiple groups
		// and then showing up in User Search.
		require.Len(t, resources, 6)
		require.NotEmpty(t, resources[0].Id)

		allIDs := mapset.NewSet[string]()
		for _, resource := range resources {
			allIDs.Add(resource.Id.Resource)
		}
		require.Equal(t, allIDs.Cardinality(), 2)
	})
}
