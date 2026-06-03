package connector

import (
	"context"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/stretchr/testify/require"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	"github.com/conductorone/baton-confluence/test"
)

// fixture role_assignments0.json has 3 assignments each with a unique roleId

func TestSpaceRoleAssignments(t *testing.T) {
	ctx := context.Background()
	server := test.FixturesServer()
	defer server.Close()

	confluenceClient, err := client.NewConfluenceClient(ctx, "username", "API Key", server.URL)
	if err != nil {
		t.Fatal(err)
	}

	b := newSpaceRoleAssignmentBuilder(confluenceClient)

	spaceResourceID := &v2.ResourceId{
		ResourceType: spaceResourceType.Id,
		Resource:     "678",
	}

	t.Run("should return nothing for nil parent", func(t *testing.T) {
		resources, results, err := b.List(ctx, nil, rs.SyncOpAttrs{})
		require.Nil(t, err)
		require.Empty(t, resources)
		require.Nil(t, results)
	})

	t.Run("should list scope binding resources for a space", func(t *testing.T) {
		resources, results, err := b.List(ctx, spaceResourceID, rs.SyncOpAttrs{})
		require.Nil(t, err)
		require.NotNil(t, results)
		test.AssertNoRatelimitAnnotations(t, results.Annotations)
		require.Equal(t, "", results.NextPageToken)
		require.Len(t, resources, 3)

		// Each resource should have the ScopeBindingTrait
		for _, res := range resources {
			scopeTrait, err := rs.GetScopeBindingTrait(res)
			require.Nil(t, err)
			require.Equal(t, "678", scopeTrait.GetScopeResourceId().GetResource())
			require.NotEmpty(t, scopeTrait.GetRoleId().GetResource())
		}

		// First resource corresponds to the first unique roleId in role_assignments0.json
		scopeTrait, _ := rs.GetScopeBindingTrait(resources[0])
		require.Equal(t, "role-001", scopeTrait.GetRoleId().GetResource())
		require.Equal(t, "Viewer on Product Management", resources[0].DisplayName)

		// Resources should be parented under the space
		require.Equal(t, spaceResourceType.Id, resources[0].ParentResourceId.ResourceType)
		require.Equal(t, "678", resources[0].ParentResourceId.Resource)
	})

	t.Run("should return one static assigned entitlement for the type", func(t *testing.T) {
		entitlements, results, err := b.StaticEntitlements(ctx, rs.SyncOpAttrs{})
		require.Nil(t, err)
		require.NotNil(t, results)
		test.AssertNoRatelimitAnnotations(t, results.Annotations)
		require.Equal(t, "", results.NextPageToken)
		require.Len(t, entitlements, 1)
		require.Equal(t, spaceRoleAssignmentEntitlement, entitlements[0].GetSlug())
	})

	t.Run("should list grants for a scope binding resource", func(t *testing.T) {
		resources, _, err := b.List(ctx, spaceResourceID, rs.SyncOpAttrs{})
		require.Nil(t, err)
		require.NotEmpty(t, resources)

		grants, results, err := b.Grants(ctx, resources[0], rs.SyncOpAttrs{})
		require.Nil(t, err)
		require.NotNil(t, results)
		test.AssertNoRatelimitAnnotations(t, results.Annotations)
		require.Equal(t, "", results.NextPageToken)
		require.NotEmpty(t, grants)

		// All grants should reference the "assigned" entitlement
		for _, g := range grants {
			require.Contains(t, g.Entitlement.Id, ":"+spaceRoleAssignmentEntitlement)
		}

		// First grant: user-123 (USER)
		require.Equal(t, "user-123", grants[0].Principal.Id.Resource)
		require.Equal(t, resourceTypeUser.Id, grants[0].Principal.Id.ResourceType)

		// Second grant: group-456 (GROUP) — should have GrantExpandable annotation
		require.Equal(t, "group-456", grants[1].Principal.Id.Resource)
		require.Equal(t, resourceTypeGroup.Id, grants[1].Principal.Id.ResourceType)
	})
}
