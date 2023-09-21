package connector

import (
	"context"

	"github.com/ConductorOne/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resource "github.com/conductorone/baton-sdk/pkg/types/resource"
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

func (o *userResourceType) List(ctx context.Context, _ *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	users, _, err := o.client.GetUsers(ctx, "", 100)
	if err != nil {
		return nil, "", nil, err
	}
	rv := make([]*v2.Resource, 0)
	for _, user := range users {
		if user.AccountType != accountTypeAtlassian {
			l.Debug("confluence: user is not of type atlassian", zap.Any("user", user))
			continue
		}
		annos := &v2.V1Identifier{
			Id: user.AccountId,
		}
		profile := userProfile(ctx, user)
		userTrait := []resource.UserTraitOption{resource.WithUserProfile(profile), resource.WithEmail(user.Email, true), resource.WithStatus(v2.UserTrait_Status_STATUS_ENABLED)}
		userResource, err := resource.NewUserResource(user.DisplayName, resourceTypeUser, user.AccountId, userTrait, resource.WithAnnotation(annos))
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, userResource)
	}

	return rv, "", nil, nil
}

func (o *userResourceType) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *userResourceType) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func userBuilder(client *client.ConfluenceClient) *userResourceType {
	return &userResourceType{
		resourceType: resourceTypeUser,
		client:       client,
	}
}

func userProfile(ctx context.Context, user client.ConfluenceUser) map[string]interface{} {
	profile := make(map[string]interface{})
	profile["user_name"] = user.DisplayName
	profile["account_type"] = user.AccountType
	profile["email"] = user.Email
	profile["id"] = user.AccountId
	return profile
}
