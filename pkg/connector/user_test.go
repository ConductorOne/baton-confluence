package connector

import (
	"context"
	"testing"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	"github.com/conductorone/baton-confluence/test"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestUsersList(t *testing.T) {
	ctx := context.Background()

	t.Run("should get users, using pagination, ignoring robots", func(t *testing.T) {
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
			pToken := pagination.Token{}
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
		// We expect there to be duplicates.
		//require.Len(t, resources, 3)
		require.Len(t, resources, 2)
		require.NotEmpty(t, resources[0].Id)
	})
}
