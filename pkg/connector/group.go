package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
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

	newGroupResource, err := resource.NewGroupResource(
		group.Name,
		resourceTypeGroup,
		group.Id,
		nil,
		resource.WithResourceProfile(profile),
	)
	if err != nil {
		return nil, err
	}

	return newGroupResource, nil
}

func (o *groupResourceType) List(
	ctx context.Context,
	resourceId *v2.ResourceId,
	opts resource.SyncOpAttrs,
) ([]*v2.Resource, *resource.SyncOpResults, error) {
	bag := &pagination.Bag{}
	err := bag.Unmarshal(opts.PageToken.Token)
	if err != nil {
		return nil, nil, err
	}
	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceTypeGroup.Id,
		})
	}
	groups, token, ratelimitData, err := o.client.GetGroups(ctx, bag.PageToken(), ResourcesPageSize)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, syncResults("", outputAnnotations), err
	}

	rv := make([]*v2.Resource, 0, len(groups))
	for _, g := range groups {
		groupCopy := g

		gr, err := groupResource(ctx, &groupCopy)
		if err != nil {
			return nil, syncResults("", outputAnnotations), err
		}

		rv = append(rv, gr)
	}

	nextPage, err := bag.NextToken(token)
	if err != nil {
		return nil, syncResults("", outputAnnotations), err
	}

	return rv, syncResults(nextPage, outputAnnotations), nil
}

func (o *groupResourceType) Entitlements(
	ctx context.Context,
	res *v2.Resource,
	_ resource.SyncOpAttrs,
) ([]*v2.Entitlement, *resource.SyncOpResults, error) {
	var rv []*v2.Entitlement

	assignmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(resourceTypeUser),
		entitlement.WithDisplayName(fmt.Sprintf("%s Group Member", res.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Is member of the %s group in Confluence", res.DisplayName)),
	}

	rv = append(rv, entitlement.NewAssignmentEntitlement(
		res,
		groupMemberEntitlement,
		assignmentOptions...,
	))

	return rv, syncResults("", nil), nil
}

func (o *groupResourceType) Grants(
	ctx context.Context,
	res *v2.Resource,
	opts resource.SyncOpAttrs,
) ([]*v2.Grant, *resource.SyncOpResults, error) {
	bag := &pagination.Bag{}
	err := bag.Unmarshal(opts.PageToken.Token)
	if err != nil {
		return nil, nil, err
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
		res.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, syncResults("", outputAnnotations), err
	}

	var rv []*v2.Grant
	for _, user := range users {
		if !shouldIncludeUser(ctx, user) {
			continue
		}

		rv = append(rv, grant.NewGrant(
			res,
			groupMemberEntitlement,
			&v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     user.AccountId,
			},
		))
	}

	nextPage, err := bag.NextToken(token)
	if err != nil {
		return nil, syncResults("", outputAnnotations), err
	}
	return rv, syncResults(nextPage, outputAnnotations), nil
}

func (o *groupResourceType) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) ([]*v2.Grant, annotations.Annotations, error) {
	ratelimitData, err := o.client.AddUserToGroup(
		ctx,
		principal.Id.Resource,
		entitlement.Resource.Id.Resource,
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, outputAnnotations, err
	}
	g := grant.NewGrant(entitlement.Resource, groupMemberEntitlement, principal.Id)
	return []*v2.Grant{g}, outputAnnotations, nil
}

func (o *groupResourceType) Revoke(
	ctx context.Context,
	grant *v2.Grant,
) (annotations.Annotations, error) {
	ratelimitData, err := o.client.RemoveUserFromGroup(
		ctx,
		grant.Principal.Id.Resource,
		grant.Entitlement.Resource.Id.Resource,
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
