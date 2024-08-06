package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const separator = "-"

func CreateEntitlementName(operation client.ConfluenceSpaceOperation) string {
	return fmt.Sprintf(
		"%s%s%s",
		operation.Operation,
		separator,
		operation.TargetType,
	)
}

// GetEntitlementComponents returns the operation and target in that order.
func GetEntitlementComponents(operation string) (string, string) {
	parts := strings.Split(operation, separator)
	return parts[0], parts[1]
}

type spaceBuilder struct {
	client client.ConfluenceClient
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
		ur, err := spaceResource(ctx, &spaceCopy)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, ur)
	}

	return rv, nextToken, outputAnnotations, nil
}

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
	logger := ctxzap.Extract(ctx)
	logger.Debug(
		"Starting call to Spaces.Entitlements",
		zap.String("resource.DisplayName", resource.DisplayName),
		zap.String("resource.Id.Resource", resource.Id.Resource),
	)
	entitlements := make([]*v2.Entitlement, 0)
	spacePermissions, nextToken, ratelimitData, err := o.client.ConfluenceSpaceOperations(
		ctx,
		pToken.Token,
		pToken.Size,
		resource.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	for _, operation := range spacePermissions {
		operationName := CreateEntitlementName(operation)
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
						"Has permission to %s the %s space in Confluence Data Center",
						operationName,
						resource.DisplayName,
					),
				),
			))
	}
	return entitlements, nextToken, outputAnnotations, nil
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
		var resourceType string
		switch permission.Principal.Type {
		case "user":
			resourceType = resourceTypeUser.Id
		case "group":
			resourceType = resourceTypeGroup.Id
		default:
			// Skip if the type is "role".
			continue
		}

		permissionName := fmt.Sprintf(
			"%s-%s",
			permission.Operation.Key,
			permission.Operation.TargetType,
		)

		permissions = append(
			permissions,
			grant.NewGrant(
				resource,
				permissionName,
				&v2.ResourceId{
					ResourceType: resourceType,
					Resource:     permission.Principal.Id,
				},
			))
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

func newSpaceBuilder(client *client.ConfluenceClient) *spaceBuilder {
	return &spaceBuilder{
		client: *client,
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
