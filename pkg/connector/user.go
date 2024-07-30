package connector

import (
	"context"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type userResourceType struct {
	resourceType *v2.ResourceType
	client       *client.ConfluenceClient
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
		b.Push(pagination.PageState{
			ResourceTypeID: resourceID.ResourceType,
			ResourceID:     resourceID.Resource,
		})
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

	bag, page, size, err := parsePageToken(
		pToken,
		&v2.ResourceId{ResourceType: resourceTypeUser.Id},
	)
	if err != nil {
		return nil, "", nil, err
	}

	users, nextToken, ratelimitData, err := o.client.GetUsersFromSearch(ctx, page, size)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}
	rv := make([]*v2.Resource, 0)
	for _, user := range users {
		if user.AccountType != accountTypeAtlassian {
			logger.Debug("confluence: user is not of type atlassian", zap.Any("user", user))
			continue
		}

		userCopy := user
		ur, err := userResource(ctx, &userCopy)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, ur)
	}
	err = bag.Next(nextToken)
	if err != nil {
		return nil, "", nil, err
	}

	nextToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return rv, nextToken, outputAnnotations, nil
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
