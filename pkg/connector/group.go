package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	"github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	groupMemberEntitlement = "member"
)

type groupResourceType struct {
	resourceType *v2.ResourceType
	client       *client.ConfluenceClient
}

func (o *groupResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func groupResource(ctx context.Context, group *client.ConfluenceGroup) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"group_id":   group.Id,
		"group_name": group.Name,
		"group_type": group.Type,
	}

	groupTraitOptions := []resource.GroupTraitOption{resource.WithGroupProfile(profile)}

	newGroupResource, err := resource.NewGroupResource(
		group.Name,
		resourceTypeGroup,
		group.Id,
		groupTraitOptions,
	)
	if err != nil {
		return nil, err
	}

	return newGroupResource, nil
}

func (o *groupResourceType) List(
	ctx context.Context,
	resourceId *v2.ResourceId,
	pt *pagination.Token,
) (
	[]*v2.Resource,
	string,
	annotations.Annotations,
	error,
) {
	bag := &pagination.Bag{}
	err := bag.Unmarshal(pt.Token)
	if err != nil {
		return nil, "", nil, err
	}
	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceTypeGroup.Id,
		})
	}
	groups, token, ratelimitData, err := o.client.GetGroups(ctx, bag.PageToken(), ResourcesPageSize)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	rv := make([]*v2.Resource, 0, len(groups))
	for _, g := range groups {
		groupCopy := g

		gr, err := groupResource(ctx, &groupCopy)
		if err != nil {
			return nil, "", outputAnnotations, err
		}

		rv = append(rv, gr)
	}

	nextPage, err := bag.NextToken(token)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	return rv, nextPage, outputAnnotations, nil
}

func (o *groupResourceType) Entitlements(
	ctx context.Context,
	resource *v2.Resource,
	_ *pagination.Token,
) (
	[]*v2.Entitlement,
	string,
	annotations.Annotations,
	error,
) {
	var rv []*v2.Entitlement

	assignmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(resourceTypeUser),
		entitlement.WithDisplayName(fmt.Sprintf("%s Group Member", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Is member of the %s group in Confluence", resource.DisplayName)),
	}

	rv = append(rv, entitlement.NewAssignmentEntitlement(
		resource,
		groupMemberEntitlement,
		assignmentOptions...,
	))

	return rv, "", nil, nil
}

func (o *groupResourceType) Grants(
	ctx context.Context,
	resource *v2.Resource,
	pt *pagination.Token,
) (
	[]*v2.Grant,
	string,
	annotations.Annotations,
	error,
) {
	l := ctxzap.Extract(ctx)
	bag := &pagination.Bag{}
	err := bag.Unmarshal(pt.Token)
	if err != nil {
		return nil, "", nil, err
	}
	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceTypeGroup.Id,
		})
	}

	users, token, ratelimitData, err := o.client.GetGroupMembers(
		ctx,
		bag.PageToken(),
		ResourcesPageSize,
		resource.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	var rv []*v2.Grant
	for _, user := range users {
		if user.AccountType != accountTypeAtlassian {
			l.Debug("confluence: user is not of type atlassian", zap.Any("user", user))
			continue
		}

		rv = append(rv, grant.NewGrant(
			resource,
			groupMemberEntitlement,
			&v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     user.AccountId,
			},
		))
	}

	nextPage, err := bag.NextToken(token)
	if err != nil {
		return nil, "", outputAnnotations, err
	}
	return rv, nextPage, outputAnnotations, nil
}

func (o *groupResourceType) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (annotations.Annotations, error) {
	ratelimitData, err := o.client.AddUserToGroup(
		ctx,
		entitlement.Resource.Id.Resource,
		principal.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	return outputAnnotations, err
}

func (o *groupResourceType) Revoke(
	ctx context.Context,
	grant *v2.Grant,
) (annotations.Annotations, error) {
	ratelimitData, err := o.client.RemoveUserFromGroup(
		ctx,
		grant.Entitlement.Resource.Id.Resource,
		grant.Principal.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	return outputAnnotations, err
}

func groupBuilder(client *client.ConfluenceClient) *groupResourceType {
	return &groupResourceType{
		resourceType: resourceTypeGroup,
		client:       client,
	}
}
