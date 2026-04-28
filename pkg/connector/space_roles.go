package connector

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type spaceRoleBuilder struct {
	client client.ConfluenceClient
}

func (b *spaceRoleBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return spaceRoleResourceType
}

func (b *spaceRoleBuilder) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID != nil {
		return nil, "", nil, nil
	}

	roles, nextCursor, rateLimitData, err := b.client.GetSpaceRoles(
		ctx,
		"",
		pToken.Token,
		ResourcesPageSize,
	)
	outputAnnotations := WithRateLimitAnnotations(rateLimitData)
	if err != nil {
		var reqErr *client.RequestError
		if errors.As(err, &reqErr) && reqErr.Status == http.StatusNotFound {
			ctxzap.Extract(ctx).Warn("confluence-connector: space roles endpoint unavailable, skipping", zap.Error(err))
			return nil, "", outputAnnotations, nil
		}
		return nil, "", outputAnnotations, fmt.Errorf("confluence-connector: failed to list space roles: %w", err)
	}

	resources := make([]*v2.Resource, 0, len(roles))
	for _, role := range roles {
		r, err := spaceRoleResource(role)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		resources = append(resources, r)
	}
	return resources, nextCursor, outputAnnotations, nil
}

func (b *spaceRoleBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *spaceRoleBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func spaceRoleResource(role client.SpaceRole) (*v2.Resource, error) {
	perms := make([]interface{}, len(role.SpacePermissions))
	for i, p := range role.SpacePermissions {
		perms[i] = p
	}
	return rs.NewRoleResource(
		role.Name,
		spaceRoleResourceType,
		role.Id,
		[]rs.RoleTraitOption{
			rs.WithRoleProfile(map[string]interface{}{
				"description":       role.Description,
				"space_permissions": perms,
			}),
		},
		rs.WithDescription(role.Description),
	)
}

func newSpaceRoleBuilder(client *client.ConfluenceClient) *spaceRoleBuilder {
	return &spaceRoleBuilder{client: *client}
}
