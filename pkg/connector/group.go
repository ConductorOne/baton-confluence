package connector

import (
	"context"
	"fmt"

	"github.com/ConductorOne/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grant "github.com/conductorone/baton-sdk/pkg/types/grant"
	res "github.com/conductorone/baton-sdk/pkg/types/resource"
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

func (o *groupResourceType) List(ctx context.Context, resourceId *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
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
	groups, token, err := o.client.GetGroups(ctx, bag.PageToken(), 100)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, 0, len(groups))
	for _, g := range groups {
		annos := &v2.V1Identifier{
			Id: g.Id,
		}
		profile := groupProfile(ctx, g)
		groupTrait := []res.GroupTraitOption{res.WithGroupProfile(profile)}
		groupResource, err := res.NewGroupResource(g.Name, resourceTypeGroup, g.Id, groupTrait, res.WithAnnotation(annos))
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, groupResource)
	}
	nextPage, err := bag.NextToken(token)
	if err != nil {
		return nil, "", nil, err
	}

	return rv, nextPage, nil, nil
}

func (o *groupResourceType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var annos annotations.Annotations
	annos.Update(&v2.V1Identifier{
		Id: V1MembershipEntitlementID(resource.Id.Resource),
	})
	member := ent.NewAssignmentEntitlement(resource, groupMemberEntitlement, ent.WithGrantableTo(resourceTypeUser))
	member.Description = fmt.Sprintf("Is member of the %s group in Confluence", resource.DisplayName)
	member.Annotations = annos
	member.DisplayName = fmt.Sprintf("%s Group Member", resource.DisplayName)
	return []*v2.Entitlement{member}, "", nil, nil
}

func (o *groupResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
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

	users, token, err := o.client.GetGroupMembers(ctx, bag.PageToken(), 100, resource.DisplayName)
	if err != nil {
		return nil, "", nil, err
	}
	var rv []*v2.Grant
	for _, user := range users {
		if user.AccountType != accountTypeAtlassian {
			l.Debug("confluence: user is not of type atlassian", zap.Any("user", user))
			continue
		}
		v1Identifier := &v2.V1Identifier{
			Id: V1GrantID(V1MembershipEntitlementID(resource.Id.Resource), user.AccountId),
		}
		gmID, err := res.NewResourceID(resourceTypeUser, user.AccountId)
		if err != nil {
			return nil, "", nil, err
		}
		grant := grant.NewGrant(resource, groupMemberEntitlement, gmID, grant.WithAnnotation(v1Identifier))
		rv = append(rv, grant)
	}

	nextPage, err := bag.NextToken(token)
	if err != nil {
		return nil, "", nil, err
	}
	return rv, nextPage, nil, nil
}

func groupBuilder(client *client.ConfluenceClient) *groupResourceType {
	return &groupResourceType{
		resourceType: resourceTypeGroup,
		client:       client,
	}
}

func groupProfile(ctx context.Context, group client.ConfluenceGroup) map[string]interface{} {
	profile := make(map[string]interface{})
	profile["group_id"] = group.Id
	profile["group_name"] = group.Name
	profile["group_type"] = group.Type
	return profile
}
