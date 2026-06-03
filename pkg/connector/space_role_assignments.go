package connector

import (
	"context"
	"encoding/json"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grantSdk "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
)

const spaceRoleAssignmentEntitlement = "assigned"

// spaceRoleListPageToken wraps the API cursor with the space display name so
// subsequent pages don't need to re-fetch it from the API.
type spaceRoleListPageToken struct {
	Cursor    string `json:"c,omitempty"`
	SpaceName string `json:"s,omitempty"`
}

type spaceRoleAssignmentBuilder struct {
	client    *client.ConfluenceClient
	roleNames map[string]string
}

func (b *spaceRoleAssignmentBuilder) loadRoleNames(ctx context.Context) error {
	// If we have 4 or more role names, we can assume we have all the role names we need.
	if len(b.roleNames) >= 4 {
		return nil
	}
	b.roleNames = make(map[string]string)
	// There can be only up to 14 roles, 4 defaults + 10 custom roles, so 1 call is enough to fetch them all.
	roles, _, _, err := b.client.GetSpaceRoles(ctx, "", "", ResourcesPageSize)
	if err != nil {
		return fmt.Errorf("confluence-connector: failed to fetch space role names: %w", err)
	}
	for _, r := range roles {
		b.roleNames[r.Id] = r.Name
	}
	return nil
}

func (b *spaceRoleAssignmentBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return spaceRoleAssignmentResourceType
}

func (b *spaceRoleAssignmentBuilder) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	opts rs.SyncOpAttrs,
) ([]*v2.Resource, *rs.SyncOpResults, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != spaceResourceType.Id {
		return nil, nil, nil
	}
	spaceID := parentResourceID.Resource

	// Decode our wrapped token to recover the API cursor and any cached space name.
	var pageToken spaceRoleListPageToken
	if opts.PageToken.Token != "" {
		if err := json.Unmarshal([]byte(opts.PageToken.Token), &pageToken); err != nil {
			// Treat as a raw API cursor for backward compatibility.
			pageToken.Cursor = opts.PageToken.Token
		}
	}

	assignments, nextCursor, rateLimitData, err := b.client.GetSpaceRoleAssignments(
		ctx, spaceID, "", "", "", pageToken.Cursor, ResourcesPageSize,
	)
	outputAnnotations := WithRateLimitAnnotations(rateLimitData)
	if err != nil {
		return nil, syncResults("", outputAnnotations), fmt.Errorf("confluence-connector: failed to list space role assignments: %w", err)
	}

	if err := b.loadRoleNames(ctx); err != nil {
		return nil, syncResults("", outputAnnotations), err
	}

	// Use the space name cached in the token when available (pages 2+).
	// On the first page it is empty, so we fetch it from the API once.
	// spaceResourceType has no SDK trait, so there is no trait to read from parentResourceID.
	spaceName := pageToken.SpaceName
	if spaceName == "" {
		spaceName = spaceID
		if space, _, err := b.client.GetSpaceById(ctx, spaceID); err == nil {
			spaceName = space.Name
		}
	}

	// Deduplicate by roleId within this page — multiple principals share a roleId.
	// Across pages, C1 deduplicates by resource ID.
	seen := make(map[string]bool)
	var resources []*v2.Resource
	for _, assignment := range assignments {
		if seen[assignment.RoleId] {
			continue
		}
		seen[assignment.RoleId] = true
		roleName := b.roleNames[assignment.RoleId]
		if roleName == "" {
			roleName = assignment.RoleId
		}
		r, err := spaceRoleAssignmentResource(assignment.RoleId, parentResourceID, roleName, spaceName)
		if err != nil {
			return nil, syncResults("", outputAnnotations), err
		}
		resources = append(resources, r)
	}

	// Encode the next token, carrying the space name forward to avoid re-fetching it.
	var nextPageToken string
	if nextCursor != "" {
		data, err := json.Marshal(spaceRoleListPageToken{Cursor: nextCursor, SpaceName: spaceName})
		if err != nil {
			return nil, syncResults("", outputAnnotations), fmt.Errorf("confluence-connector: failed to marshal page token: %w", err)
		}
		nextPageToken = string(data)
	}

	return resources, syncResults(nextPageToken, outputAnnotations), nil
}

func (b *spaceRoleAssignmentBuilder) StaticEntitlements(
	_ context.Context,
	_ rs.SyncOpAttrs,
) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			nil,
			spaceRoleAssignmentEntitlement,
			entitlement.WithGrantableTo(resourceTypeUser, resourceTypeGroup),
		),
	}, syncResults("", nil), nil
}

func (b *spaceRoleAssignmentBuilder) Entitlements(
	_ context.Context,
	_ *v2.Resource,
	_ rs.SyncOpAttrs,
) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func (b *spaceRoleAssignmentBuilder) Grants(
	ctx context.Context,
	res *v2.Resource,
	opts rs.SyncOpAttrs,
) ([]*v2.Grant, *rs.SyncOpResults, error) {
	scopeTrait, err := rs.GetScopeBindingTrait(res)
	if err != nil {
		return nil, nil, fmt.Errorf("confluence-connector: failed to get scope binding trait: %w", err)
	}
	if scopeTrait == nil {
		return nil, nil, fmt.Errorf("confluence-connector: scope binding trait was not found on resource")
	}

	spaceID := scopeTrait.GetScopeResourceId().GetResource()
	roleID := scopeTrait.GetRoleId().GetResource()

	assignments, nextCursor, rateLimitData, err := b.client.GetSpaceRoleAssignments(
		ctx, spaceID, roleID, "", "", opts.PageToken.Token, opts.PageToken.Size,
	)
	outputAnnotations := WithRateLimitAnnotations(rateLimitData)
	if err != nil {
		return nil, syncResults("", outputAnnotations), fmt.Errorf("confluence-connector: failed to list space role assignments: %w", err)
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
			spaceRoleAssignmentEntitlement,
			&v2.ResourceId{
				ResourceType: resourceType,
				Resource:     assignment.Principal.PrincipalId,
			},
			grantOpts...,
		))
	}
	return grants, syncResults(nextCursor, outputAnnotations), nil
}

func (b *spaceRoleAssignmentBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	ent *v2.Entitlement,
) ([]*v2.Grant, annotations.Annotations, error) {
	resource := ent.GetResource()
	if resource == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "confluence-connector: entitlement resource is nil")
	}
	scopeTrait, err := rs.GetScopeBindingTrait(resource)
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "confluence-connector: failed to get scope binding trait: %v", err)
	}
	if scopeTrait == nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "confluence-connector: scope binding trait was not found on resource")
	}
	spaceID := scopeTrait.GetScopeResourceId().GetResource()
	roleID := scopeTrait.GetRoleId().GetResource()

	principalType, err := confluencePrincipalType(principal.Id.ResourceType)
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "confluence-connector: %v", err)
	}
	principalID := principal.Id.Resource

	existing, _, _, err := b.client.GetSpaceRoleAssignments(ctx, spaceID, roleID, principalID, principalType, "", 1)
	if err != nil {
		return nil, nil, fmt.Errorf("confluence-connector: failed to check existing role assignments: %w", err)
	}
	if len(existing) > 0 {
		return nil, annotations.New(&v2.GrantAlreadyExists{}), nil
	}

	ratelimitData, err := b.client.SetSpaceRoleAssignment(
		ctx,
		spaceID,
		[]client.SetSpaceRoleAssignmentRequest{
			{
				Principal: client.SpaceRoleAssignmentPrincipal{
					PrincipalType: principalType,
					PrincipalId:   principalID,
				},
				RoleId: roleID,
			},
		},
	)
	outputAnnotations := WithRateLimitAnnotations(ratelimitData)
	if err != nil {
		return nil, outputAnnotations, fmt.Errorf("confluence-connector: failed to grant space role: %w", err)
	}
	g := grantSdk.NewGrant(resource, spaceRoleAssignmentEntitlement, principal.Id)
	return []*v2.Grant{g}, outputAnnotations, nil
}

func (b *spaceRoleAssignmentBuilder) Revoke(
	ctx context.Context,
	grant *v2.Grant,
) (annotations.Annotations, error) {
	resource := grant.GetEntitlement().GetResource()
	if resource == nil {
		return nil, status.Error(codes.InvalidArgument, "confluence-connector: grant entitlement resource is nil")
	}
	scopeTrait, err := rs.GetScopeBindingTrait(resource)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "confluence-connector: failed to get scope binding trait: %v", err)
	}
	if scopeTrait == nil {
		return nil, status.Error(codes.InvalidArgument, "confluence-connector: scope binding trait was not found on resource")
	}
	spaceID := scopeTrait.GetScopeResourceId().GetResource()
	roleID := scopeTrait.GetRoleId().GetResource()

	principalType, err := confluencePrincipalType(grant.Principal.Id.ResourceType)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "confluence-connector: %v", err)
	}
	principalID := grant.Principal.Id.Resource

	existing, _, _, err := b.client.GetSpaceRoleAssignments(ctx, spaceID, roleID, principalID, principalType, "", 1)
	if err != nil {
		return nil, fmt.Errorf("confluence-connector: failed to check existing role assignments: %w", err)
	}
	if len(existing) == 0 {
		return annotations.New(&v2.GrantAlreadyRevoked{}), nil
	}

	ratelimitData, err := b.client.SetSpaceRoleAssignment(
		ctx,
		spaceID,
		[]client.SetSpaceRoleAssignmentRequest{
			{
				Principal: client.SpaceRoleAssignmentPrincipal{
					PrincipalType: principalType,
					PrincipalId:   principalID,
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

func spaceRoleAssignmentResource(roleId string, spaceResourceID *v2.ResourceId, roleName, spaceName string) (*v2.Resource, error) {
	return rs.NewScopeBindingResource(
		fmt.Sprintf("%s on %s", roleName, spaceName),
		spaceRoleAssignmentResourceType,
		fmt.Sprintf("%s:%s", spaceResourceID.Resource, roleId),
		[]rs.ScopeBindingTraitOption{
			rs.WithRoleScopeRoleId(&v2.ResourceId{
				ResourceType: spaceRoleResourceType.Id,
				Resource:     roleId,
			}),
			rs.WithRoleScopeResourceId(spaceResourceID),
		},
		rs.WithParentResourceID(spaceResourceID),
	)
}

func newSpaceRoleAssignmentBuilder(client *client.ConfluenceClient) *spaceRoleAssignmentBuilder {
	return &spaceRoleAssignmentBuilder{client: client}
}
