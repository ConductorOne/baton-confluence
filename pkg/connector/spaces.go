package connector

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grantSdk "github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
)

const separator = "-"

func createEntitlementName(verb, noun string) string {
	return fmt.Sprintf(
		"%s%s%s",
		verb,
		separator,
		noun,
	)
}

// GetEntitlementComponents returns the operation and target in that order.
func GetEntitlementComponents(operation string) (string, string) {
	parts := strings.Split(operation, separator)
	return parts[0], parts[1]
}

type spaceBuilder struct {
	client             client.ConfluenceClient
	skipPersonalSpaces bool
}

func (o *spaceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return spaceResourceType
}

// List returns all the spaces from the database as resource objects.
func (o *spaceBuilder) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) (
	[]*v2.Resource,
	string,
	annotations.Annotations,
	error,
) {
	spaces, nextToken, ratelimitData, err := o.client.GetSpaces(
		ctx,
		ResourcesPageSize,
		pToken.Token,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}
	rv := make([]*v2.Resource, 0)
	for _, space := range spaces {
		spaceCopy := space
		if o.skipPersonalSpaces && spaceCopy.Type == "personal" {
			continue
		}
		ur, err := spaceResource(ctx, &spaceCopy)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, ur)
	}

	return rv, nextToken, outputAnnotations, nil
}

var allNouns = []string{
	"attachment",
	"blogpost",
	"comment",
	"page",
	"space",
}
var allVerbs = []string{
	"administer",
	"archive",
	"create",
	"delete",
	"export",
	"read",
	"restrict_content",
	"update",
}

var nounSet = mapset.NewSet[string](allNouns...)
var verbSet = mapset.NewSet[string](allVerbs...)

func (o *spaceBuilder) Entitlements(
	ctx context.Context,
	resource *v2.Resource,
	pToken *pagination.Token,
) (
	[]*v2.Entitlement,
	string,
	annotations.Annotations,
	error,
) {
	entitlements := make([]*v2.Entitlement, 0)

	// Confluence's API doesn't list all the operations you can do on a space, so we use a hard-coded list

	for _, noun := range allNouns {
		for _, verb := range allVerbs {
			operationName := createEntitlementName(verb, noun)
			entitlements = append(
				entitlements,
				entitlement.NewPermissionEntitlement(
					resource,
					operationName,
					entitlement.WithGrantableTo(resourceTypeUser),
					entitlement.WithGrantableTo(resourceTypeGroup),
					entitlement.WithDisplayName(
						fmt.Sprintf(
							"Can %s %s",
							operationName,
							resource.DisplayName,
						),
					),
					entitlement.WithDescription(
						fmt.Sprintf(
							"Has permission to %s %s the %s space in Confluence",
							verb,
							noun,
							resource.DisplayName,
						),
					),
				))
		}
	}

	return entitlements, "", nil, nil
}

// checkSpacePermission checks if the operation is in the list of operations we care about.
// Confluence's API doesn't list all the operations you can do on a space, so we use a hard-coded list.
func checkSpacePermission(operation, targetType string) bool {
	return verbSet.Contains(operation) && nounSet.Contains(targetType)
}

// Grants the grants for a given space are the permissions.
func (o *spaceBuilder) Grants(
	ctx context.Context,
	resource *v2.Resource,
	pToken *pagination.Token,
) (
	[]*v2.Grant,
	string,
	annotations.Annotations,
	error,
) {
	permissionsList, nextToken, ratelimitData, err := o.client.GetSpacePermissions(
		ctx,
		pToken.Token,
		pToken.Size,
		resource.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	var permissions []*v2.Grant
	for _, permission := range permissionsList {
		grantOpts := []grantSdk.GrantOption{}
		var resourceType string
		switch permission.Principal.Type {
		case "user":
			resourceType = resourceTypeUser.Id
		case "group":
			resourceType = resourceTypeGroup.Id
			grantOpts = append(grantOpts, grantSdk.WithAnnotation(&v2.GrantExpandable{
				EntitlementIds: []string{
					fmt.Sprintf("group:%s:member", permission.Principal.Id),
				},
			}))
		default:
			// Skip if the type is "role".
			continue
		}

		if !checkSpacePermission(permission.Operation.Key, permission.Operation.TargetType) {
			continue
		}

		grant := grantSdk.NewGrant(
			resource,
			createEntitlementName(permission.Operation.Key, permission.Operation.TargetType),
			&v2.ResourceId{
				ResourceType: resourceType,
				Resource:     permission.Principal.Id,
			},
			grantOpts...,
		)
		permissions = append(permissions, grant)
	}

	return permissions, nextToken, outputAnnotations, nil
}

func (o *spaceBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (annotations.Annotations, error) {
	spaceName := entitlement.Resource.Id.Resource
	key, target := GetEntitlementComponents(entitlement.Slug)
	ratelimitData, err := o.client.AddSpacePermission(
		ctx,
		spaceName,
		key,
		target,
		principal.Id.Resource,
		principal.Id.ResourceType,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	return outputAnnotations, err
}

func (o *spaceBuilder) Revoke(
	ctx context.Context,
	grant *v2.Grant,
) (annotations.Annotations, error) {
	spaceId := grant.Entitlement.Resource.Id.Resource
	key, target := GetEntitlementComponents(grant.Entitlement.Slug)
	ratelimitData, err := o.client.RemoveSpacePermission(
		ctx,
		spaceId,
		key,
		target,
		grant.Principal.Id.Resource,
		grant.Principal.Id.ResourceType,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	return outputAnnotations, err
}

func newSpaceBuilder(client *client.ConfluenceClient, skipPersonalSpaces bool) *spaceBuilder {
	return &spaceBuilder{
		client:             *client,
		skipPersonalSpaces: skipPersonalSpaces,
	}
}

func spaceResource(ctx context.Context, space *client.ConfluenceSpace) (*v2.Resource, error) {
	createdResource, err := resource.NewResource(
		space.Name,
		spaceResourceType,
		space.Id,
	)
	if err != nil {
		return nil, err
	}

	return createdResource, nil
}
