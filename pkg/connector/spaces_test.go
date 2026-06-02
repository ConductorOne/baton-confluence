package connector

import (
	"context"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
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
		false,
		[]string{
			"attachment",
			"blogpost",
			"comment",
			"page",
			resourceTypeSpaceID,
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
			nextResources, results, err := c.List(ctx, nil, resource.SyncOpAttrs{PageToken: pToken})
			resources = append(resources, nextResources...)

			require.Nil(t, err)
			require.NotNil(t, results)
			test.AssertNoRatelimitAnnotations(t, results.Annotations)
			if results.NextPageToken == "" {
				break
			}
			pToken.Token = results.NextPageToken
		}

		require.Len(t, resources, 2)
		require.NotEmpty(t, resources[0].Id)
	})

	t.Run("should list grants for a space", func(t *testing.T) {
		confluenceSpace := client.ConfluenceSpace{
			Id: "678",
		}
		space, _ := spaceResource(ctx, &confluenceSpace, false)

		grants, results, err := c.Grants(ctx, space, resource.SyncOpAttrs{})
		require.Nil(t, err)
		require.NotNil(t, results)
		test.AssertNoRatelimitAnnotations(t, results.Annotations)
		require.Equal(t, "", results.NextPageToken)
		require.Len(t, grants, 25)
	})
}
