package connector

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grantSdk "github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
)

const separator = "-"

func createEntitlementName(verb, noun string) string {
	return fmt.Sprintf("%s%s%s", verb, separator, noun)
}

// GetEntitlementComponents returns the operation and target in that order.
func GetEntitlementComponents(operation string) (string, string) {
	parts := strings.Split(operation, separator)
	return parts[0], parts[1]
}

type spaceBuilder struct {
	client             *client.ConfluenceClient
	skipPersonalSpaces bool
	useRbac            bool
	nouns              []string
	verbs              []string
}

func (o *spaceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return spaceResourceType
}

// List returns all the spaces as resource objects.
func (o *spaceBuilder) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	opts resource.SyncOpAttrs,
) ([]*v2.Resource, *resource.SyncOpResults, error) {
	spaces, nextToken, ratelimitData, err := o.client.GetSpaces(
		ctx,
		ResourcesPageSize,
		opts.PageToken.Token,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, syncResults("", outputAnnotations), err
	}
	rv := make([]*v2.Resource, 0)
	for _, space := range spaces {
		spaceCopy := space
		if o.skipPersonalSpaces && spaceCopy.Type == "personal" {
			continue
		}
		ur, err := spaceResource(ctx, &spaceCopy, o.useRbac)
		if err != nil {
			return nil, nil, err
		}
		rv = append(rv, ur)
	}

	return rv, syncResults(nextToken, outputAnnotations), nil
}

func (o *spaceBuilder) Entitlements(
	ctx context.Context,
	res *v2.Resource,
	_ resource.SyncOpAttrs,
) ([]*v2.Entitlement, *resource.SyncOpResults, error) {
	if o.useRbac {
		return nil, nil, nil
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
						fmt.Sprintf("Can %s %s", operationName, res.DisplayName),
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

	return entitlements, syncResults("", nil), nil
}

// checkSpacePermission checks if the operation is in the list of operations we care about.
func checkSpacePermission(nouns mapset.Set[string], verbs mapset.Set[string], operation, targetType string) bool {
	return verbs.Contains(operation) && nouns.Contains(targetType)
}

func (o *spaceBuilder) Grants(
	ctx context.Context,
	res *v2.Resource,
	opts resource.SyncOpAttrs,
) ([]*v2.Grant, *resource.SyncOpResults, error) {
	if o.useRbac {
		return nil, nil, nil
	}

	permissionsList, nextToken, ratelimitData, err := o.client.GetSpacePermissions(
		ctx,
		opts.PageToken.Token,
		opts.PageToken.Size,
		res.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, syncResults("", outputAnnotations), err
	}

	nounsSet := mapset.NewSet(o.nouns...)
	verbsSet := mapset.NewSet(o.verbs...)

	var grants []*v2.Grant
	for _, permission := range permissionsList {
		var grantOpts []grantSdk.GrantOption
		var resourceType string
		switch permission.Principal.Type {
		case resourceTypeUserID:
			resourceType = resourceTypeUser.Id
		case resourceTypeGroupID:
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

	return grants, syncResults(nextToken, outputAnnotations), nil
}

func (o *spaceBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	ent *v2.Entitlement,
) ([]*v2.Grant, annotations.Annotations, error) {
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
	if err != nil {
		return nil, outputAnnotations, err
	}
	g := grantSdk.NewGrant(ent.Resource, ent.Slug, principal.Id)
	return []*v2.Grant{g}, outputAnnotations, nil
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

func newSpaceBuilder(client *client.ConfluenceClient, skipPersonalSpaces bool, useRbac bool, nouns, verbs []string) *spaceBuilder {
	return &spaceBuilder{
		client:             client,
		skipPersonalSpaces: skipPersonalSpaces,
		useRbac:            useRbac,
		nouns:              nouns,
		verbs:              verbs,
	}
}

func spaceResource(ctx context.Context, space *client.ConfluenceSpace, useRbac bool) (*v2.Resource, error) {
	var opts []resource.ResourceOption
	if useRbac {
		opts = append(opts, resource.WithAnnotation(&v2.ChildResourceType{
			ResourceTypeId: spaceRoleAssignmentResourceType.Id,
		}))
	}
	return resource.NewResource(space.Name, spaceResourceType, space.Id, opts...)
}
