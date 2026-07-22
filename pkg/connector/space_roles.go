package connector

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type spaceRoleBuilder struct {
	client *client.ConfluenceClient
}

func (b *spaceRoleBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return spaceRoleResourceType
}

func (b *spaceRoleBuilder) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	opts rs.SyncOpAttrs,
) ([]*v2.Resource, *rs.SyncOpResults, error) {
	if parentResourceID != nil {
		return nil, nil, nil
	}

	roles, nextCursor, rateLimitData, err := b.client.GetSpaceRoles(
		ctx,
		"",
		opts.PageToken.Token,
		ResourcesPageSize,
	)
	outputAnnotations := WithRateLimitAnnotations(rateLimitData)
	if err != nil {
		var reqErr *client.RequestError
		if errors.As(err, &reqErr) && reqErr.Status == http.StatusNotFound {
			ctxzap.Extract(ctx).Warn("confluence-connector: space roles endpoint unavailable, skipping", zap.Error(err))
			return nil, syncResults("", outputAnnotations), nil
		}
		return nil, syncResults("", outputAnnotations), fmt.Errorf("confluence-connector: failed to list space roles: %w", err)
	}

	resources := make([]*v2.Resource, 0, len(roles))
	for _, role := range roles {
		r, err := spaceRoleResource(role)
		if err != nil {
			return nil, syncResults("", outputAnnotations), err
		}
		resources = append(resources, r)
	}
	return resources, syncResults(nextCursor, outputAnnotations), nil
}

func (b *spaceRoleBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func (b *spaceRoleBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
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
		nil,
		rs.WithDescription(role.Description),
		rs.WithResourceProfile(map[string]interface{}{
			"description":       role.Description,
			"space_permissions": perms,
		}),
	)
}

func newSpaceRoleBuilder(client *client.ConfluenceClient) *spaceRoleBuilder {
	return &spaceRoleBuilder{client: client}
}
