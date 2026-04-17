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
	useRbac            bool
	nouns              []string
	verbs              []string
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

func (o *spaceBuilder) Entitlements(
	ctx context.Context,
	res *v2.Resource,
	pToken *pagination.Token,
) (
	[]*v2.Entitlement,
	string,
	annotations.Annotations,
	error,
) {
	if o.useRbac {
		roles, nextCursor, ratelimitData, err := o.client.GetSpaceRoles(
			ctx,
			"",
			pToken.Token,
			ResourcesPageSize,
		)
		outputAnnotations := WithRateLimitAnnotations(ratelimitData)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("confluence-connector: failed to list space roles: %w", err)
		}

		var entitlements []*v2.Entitlement
		for _, role := range roles {
			entitlements = append(entitlements, NewPermissionEntitlement(
				res,
				role.Id,
				role.Name,
				entitlement.WithGrantableTo(resourceTypeUser, resourceTypeGroup),
				entitlement.WithDisplayName(fmt.Sprintf("%s role", role.Name)),
				entitlement.WithDescription(fmt.Sprintf("Has the %s role in Confluence space %s", role.Name, res.DisplayName)),
			))
		}
		return entitlements, nextCursor, outputAnnotations, nil
	}

	entitlements := make([]*v2.Entitlement, 0)

	for _, noun := range o.nouns {
		for _, verb := range o.verbs {
			operationName := createEntitlementName(verb, noun)
			entitlements = append(
				entitlements,
				entitlement.NewPermissionEntitlement(
					res,
					operationName,
					entitlement.WithGrantableTo(resourceTypeUser),
					entitlement.WithGrantableTo(resourceTypeGroup),
					entitlement.WithDisplayName(
						fmt.Sprintf(
							"Can %s %s",
							operationName,
							res.DisplayName,
						),
					),
					entitlement.WithDescription(
						fmt.Sprintf(
							"Has permission to %s %s the %s space in Confluence",
							verb,
							noun,
							res.DisplayName,
						),
					),
				))
		}
	}

	return entitlements, "", nil, nil
}

// checkSpacePermission checks if the operation is in the list of operations we care about.
func checkSpacePermission(nouns mapset.Set[string], verbs mapset.Set[string], operation, targetType string) bool {
	return verbs.Contains(operation) && nouns.Contains(targetType)
}

func (o *spaceBuilder) Grants(
	ctx context.Context,
	res *v2.Resource,
	pToken *pagination.Token,
) (
	[]*v2.Grant,
	string,
	annotations.Annotations,
	error,
) {
	if o.useRbac {
		assignments, nextCursor, ratelimitData, err := o.client.GetSpaceRoleAssignments(
			ctx,
			res.Id.Resource,
			"",
			"",
			"",
			pToken.Token,
			pToken.Size,
		)
		outputAnnotations := WithRateLimitAnnotations(ratelimitData)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("confluence-connector: failed to list space role assignments: %w", err)
		}

		var grants []*v2.Grant
		for _, assignment := range assignments {
			var resourceType string
			var grantOpts []grantSdk.GrantOption

			switch assignment.Principal.PrincipalType {
			case "USER":
				resourceType = resourceTypeUser.Id
			case "GROUP":
				resourceType = resourceTypeGroup.Id
				grantOpts = append(grantOpts, grantSdk.WithAnnotation(&v2.GrantExpandable{
					EntitlementIds: []string{
						fmt.Sprintf("group:%s:member", assignment.Principal.PrincipalId),
					},
				}))
			default:
				continue
			}

			grants = append(grants, grantSdk.NewGrant(
				res,
				assignment.RoleId,
				&v2.ResourceId{
					ResourceType: resourceType,
					Resource:     assignment.Principal.PrincipalId,
				},
				grantOpts...,
			))
		}

		return grants, nextCursor, outputAnnotations, nil
	}

	permissionsList, nextToken, ratelimitData, err := o.client.GetSpacePermissions(
		ctx,
		pToken.Token,
		pToken.Size,
		res.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	nounsSet := mapset.NewSet(o.nouns...)
	verbsSet := mapset.NewSet(o.verbs...)

	var grants []*v2.Grant
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
			continue
		}
		if !checkSpacePermission(nounsSet, verbsSet, permission.Operation.Key, permission.Operation.TargetType) {
			continue
		}
		grants = append(grants, grantSdk.NewGrant(
			res,
			createEntitlementName(permission.Operation.Key, permission.Operation.TargetType),
			&v2.ResourceId{ResourceType: resourceType, Resource: permission.Principal.Id},
			grantOpts...,
		))
	}

	return grants, nextToken, outputAnnotations, nil
}

func (o *spaceBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	ent *v2.Entitlement,
) (annotations.Annotations, error) {
	if o.useRbac {
		spaceId := ent.Resource.Id.Resource
		parts := strings.SplitN(ent.Id, ":", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("confluence-connector: invalid entitlement ID: %q", ent.Id)
		}
		roleId := parts[2]

		principalType, err := confluencePrincipalType(principal.Id.ResourceType)
		if err != nil {
			return nil, fmt.Errorf("confluence-connector: %w", err)
		}

		principalId := principal.Id.Resource

		existing, _, _, err := o.client.GetSpaceRoleAssignments(ctx, spaceId, roleId, principalId, principalType, "", 1)
		if err != nil {
			return nil, fmt.Errorf("confluence-connector: failed to check existing role assignments: %w", err)
		}
		if len(existing) > 0 {
			return annotations.New(&v2.GrantAlreadyExists{}), nil
		}

		ratelimitData, err := o.client.SetSpaceRoleAssignment(
			ctx,
			spaceId,
			[]client.SetSpaceRoleAssignmentRequest{
				{
					Principal: client.SpaceRoleAssignmentPrincipal{
						PrincipalType: principalType,
						PrincipalId:   principalId,
					},
					RoleId: roleId,
				},
			},
		)
		outputAnnotations := WithRateLimitAnnotations(ratelimitData)
		if err != nil {
			return outputAnnotations, fmt.Errorf("confluence-connector: failed to grant space role: %w", err)
		}
		return outputAnnotations, nil
	}

	spaceName := ent.Resource.Id.Resource
	key, target := GetEntitlementComponents(ent.Slug)
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
	if o.useRbac {
		spaceId := grant.Entitlement.Resource.Id.Resource
		parts := strings.SplitN(grant.Entitlement.Id, ":", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("confluence-connector: invalid entitlement ID: %q", grant.Entitlement.Id)
		}
		roleId := parts[2]

		principalType, err := confluencePrincipalType(grant.Principal.Id.ResourceType)
		if err != nil {
			return nil, fmt.Errorf("confluence-connector: %w", err)
		}

		principalId := grant.Principal.Id.Resource

		existing, _, _, err := o.client.GetSpaceRoleAssignments(ctx, spaceId, roleId, principalId, principalType, "", 1)
		if err != nil {
			return nil, fmt.Errorf("confluence-connector: failed to check existing role assignments: %w", err)
		}
		if len(existing) == 0 {
			return annotations.New(&v2.GrantAlreadyRevoked{}), nil
		}

		ratelimitData, err := o.client.SetSpaceRoleAssignment(
			ctx,
			spaceId,
			[]client.SetSpaceRoleAssignmentRequest{
				{
					Principal: client.SpaceRoleAssignmentPrincipal{
						PrincipalType: principalType,
						PrincipalId:   principalId,
					},
					// RoleId intentionally omitted: signals removal to the API
				},
			},
		)
		outputAnnotations := WithRateLimitAnnotations(ratelimitData)
		if err != nil {
			return outputAnnotations, fmt.Errorf("confluence-connector: failed to revoke space role: %w", err)
		}
		return outputAnnotations, nil
	}

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

func newSpaceBuilder(client *client.ConfluenceClient, skipPersonalSpaces bool, useRbac bool, nouns, verbs []string) *spaceBuilder {
	return &spaceBuilder{
		client:             *client,
		skipPersonalSpaces: skipPersonalSpaces,
		useRbac:            useRbac,
		nouns:              nouns,
		verbs:              verbs,
	}
}

func spaceResource(ctx context.Context, space *client.ConfluenceSpace) (*v2.Resource, error) {
	return resource.NewResource(
		space.Name,
		spaceResourceType,
		space.Id,
	)
}
