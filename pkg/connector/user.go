package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	GroupPageSizeMaximum = 25
)

type userResourceType struct {
	resourceType *v2.ResourceType
	client       *client.ConfluenceClient
}

// limitPageSizeForGroups - Enforcing an arbitrarily small page size limit for
// groups so that we can handle the 2D pagination scheme with a comfortable margin.
func limitPageSizeForGroups(pageSize int) int {
	if pageSize <= 0 || pageSize > GroupPageSizeMaximum {
		return GroupPageSizeMaximum
	}
	return pageSize
}

func (o *userResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func userResource(ctx context.Context, user *client.ConfluenceUser) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"user_name":    user.DisplayName,
		"account_type": user.AccountType,
		"email":        user.Email,
		"id":           user.AccountId,
	}

	userTraitOptions := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithEmail(user.Email, true),
		resource.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
	}

	newUserResource, err := resource.NewUserResource(
		user.DisplayName,
		resourceTypeUser,
		user.AccountId,
		userTraitOptions,
	)
	if err != nil {
		return nil, err
	}

	return newUserResource, nil
}

// parsePageToken given a marshalled pageToken as a string, return the pageToken
// bag and the current page number.
func parsePageToken(
	pToken *pagination.Token,
	resourceID *v2.ResourceId,
) (
	*pagination.Bag,
	string,
	int,
	error,
) {
	b := &pagination.Bag{}
	err := b.Unmarshal(pToken.Token)
	if err != nil {
		return nil, "0", 0, err
	}

	if b.Current() == nil {
		b.Push(
			pagination.PageState{
				ResourceTypeID: resourceID.ResourceType,
				ResourceID:     resourceID.Resource,
			},
		)
	}

	page := b.PageToken()
	size := pToken.Size
	if size == 0 {
		size = ResourcesPageSize
	}
	if page == "" {
		page = "0"
	}
	return b, page, size, nil
}

func (o *userResourceType) List(
	ctx context.Context,
	_ *v2.ResourceId,
	pToken *pagination.Token,
) (
	[]*v2.Resource,
	string,
	annotations.Annotations,
	error,
) {
	logger := ctxzap.Extract(ctx)
	logger.Debug("Starting Users List", zap.String("token", pToken.Token))

	// There is no Confluence Cloud REST API to get all users, so get all groups
	// and then all members of each group.

	// The second parameter here is "user", which acts as a default value.
	bag, page, size, err := parsePageToken(
		pToken,
		&v2.ResourceId{ResourceType: resourceTypeUser.Id},
	)
	if err != nil {
		return nil, "", nil, err
	}

	outputResources := make([]*v2.Resource, 0)
	var outputAnnotations annotations.Annotations
	switch bag.ResourceTypeID() {
	case resourceTypeUser.Id:
		logger.Debug("Got a user from the bag", zap.String("page", page))
		if page == "" {
			page = "0"
		}

		size = limitPageSizeForGroups(size)

		// Add a new page of groups
		groups, nextToken, ratelimitData, err := o.client.GetGroups(
			ctx,
			page,
			size,
		)
		logger.Debug(
			"Got groups",
			zap.Int("len", len(groups)),
			zap.String("nextToken", nextToken),
		)
		outputAnnotations = WithRateLimitAnnotations(ratelimitData)
		if err != nil {
			return nil, "", outputAnnotations, err
		}

		// Push next page to stack. (Short-circuits if token is "".)
		err = bag.Next(nextToken)
		if err != nil {
			return nil, "", outputAnnotations, err
		}

		for _, group := range groups {
			logger.Debug(
				"adding a group to the bag",
				zap.String("id", group.Id),
				zap.String("name", group.Name),
			)
			bag.Push(
				pagination.PageState{
					ResourceTypeID: resourceTypeGroup.Id,
					ResourceID:     group.Id,
				},
			)
		}
	case resourceTypeGroup.Id:
		currentState := bag.Current()

		start := currentState.Token
		if start == "" {
			start = "0"
		}
		logger.Debug(
			"Got a group from the bag",
			zap.String("start", start),
			zap.String("group_id", currentState.ResourceID),
		)

		// Get users for this group.
		users, nextToken, ratelimitData, err := o.client.GetGroupMembers(
			ctx,
			start,
			size,
			currentState.ResourceID,
		)
		outputAnnotations = WithRateLimitAnnotations(ratelimitData)
		if err != nil {
			return nil, "", outputAnnotations, err
		}

		// Push next page to stack. (Short-circuits if token is "".)
		err = bag.Next(nextToken)
		if err != nil {
			return nil, "", outputAnnotations, err
		}

		// Add users to output resources. There will be duplicates across groups.
		for _, user := range users {
			if user.AccountType != accountTypeAtlassian {
				logger.Debug("confluence: user is not of type atlassian", zap.Any("user", user))
				continue
			}

			userCopy := user
			newUserResource, err := userResource(ctx, &userCopy)
			if err != nil {
				return nil, "", nil, err
			}

			outputResources = append(outputResources, newUserResource)
		}
	default:
		return nil, "", nil, fmt.Errorf("unexpected resource type while fetching list of users")
	}

	pageToken, err := bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return outputResources, pageToken, outputAnnotations, nil
}

func (o *userResourceType) Entitlements(
	_ context.Context,
	_ *v2.Resource,
	_ *pagination.Token,
) (
	[]*v2.Entitlement,
	string,
	annotations.Annotations,
	error,
) {
	return nil, "", nil, nil
}

func (o *userResourceType) Grants(
	_ context.Context,
	_ *v2.Resource,
	_ *pagination.Token,
) (
	[]*v2.Grant,
	string,
	annotations.Annotations,
	error,
) {
	return nil, "", nil, nil
}

func userBuilder(client *client.ConfluenceClient) *userResourceType {
	return &userResourceType{
		resourceType: resourceTypeUser,
		client:       client,
	}
}
