package connector

import (
	"context"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/stretchr/testify/require"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	"github.com/conductorone/baton-confluence/test"
)

func TestSpaces(t *testing.T) {
	ctx := context.Background()
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

	c := newSpaceBuilder(
		confluenceClient,
		false,
 		[]string{
			"attachment",
			"blogpost",
			"comment",
			"page",
			"space",
		},
		[]string{
			"administer",
			"archive",
			"create",
			"delete",
			"export",
			"read",
			"restrict_content",
			"update",
		},
	)

	t.Run("should list spaces", func(t *testing.T) {
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

		require.Nil(t, err)
		require.Len(t, resources, 2)
		require.NotEmpty(t, resources[0].Id)
	})

	t.Run("should list grants for a space", func(t *testing.T) {
		confluenceSpace := client.ConfluenceSpace{
			Id: "678",
		}
		space, _ := spaceResource(ctx, &confluenceSpace)

		grants, nextToken, grantsAnnotations, err := c.Grants(ctx, space, &pagination.Token{})
		require.Nil(t, err)
		test.AssertNoRatelimitAnnotations(t, grantsAnnotations)
		require.Equal(t, "", nextToken)
		require.Len(t, grants, 25)
	})
}
