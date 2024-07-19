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

func TestGroups(t *testing.T) {
	ctx := context.Background()

	server := test.FixturesServer()

	confluenceClient, err := client.NewConfluenceClient(
		ctx,
		"username",
		"API Key",
		server.URL,
	)

	if err != nil {
		t.Fatal(err)
	}

	c := groupBuilder(confluenceClient)

	t.Run("should list groups", func(t *testing.T) {
		resources := make([]*v2.Resource, 0)
		pToken := pagination.Token{}
		for {
			nextResources, nextToken, listAnnotations, err := c.List(ctx, nil, &pToken)
			resources = append(resources, nextResources...)

			require.Nil(t, err)
			test.AssertNoRatelimitAnnotations(t, listAnnotations)
			if nextToken == "" {
				break
			}
			pToken.Token = nextToken
		}

		require.Len(t, resources, 2)
		require.NotEmpty(t, resources[0].Id)
	})

	t.Run("should list grants for a group", func(t *testing.T) {
		confluenceGroup := client.ConfluenceGroup{
			Id: "456",
		}
		group, _ := groupResource(ctx, &confluenceGroup)

		grants := make([]*v2.Grant, 0)
		pToken := pagination.Token{}
		for {
			nextGrants, nextToken, grantAnnotations, err := c.Grants(ctx, group, &pToken)
			grants = append(grants, nextGrants...)

			require.Nil(t, err)
			test.AssertNoRatelimitAnnotations(t, grantAnnotations)
			if nextToken == "" {
				break
			}
			pToken.Token = nextToken
		}
		require.Len(t, grants, 2)
		require.NotEmpty(t, grants[0].Id)
	})
}
