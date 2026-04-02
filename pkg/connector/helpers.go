package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

// ResourcesPageSize controls the default page size for v1 API list operations.
// Confluence Cloud v1 API defaults to 1000 with no documented hard maximum.
const ResourcesPageSize = 500

func annotationsForUserResourceType() annotations.Annotations {
	annos := annotations.Annotations{}
	annos.Update(&v2.SkipEntitlementsAndGrants{})
	return annos
}

func WithRateLimitAnnotations(
	ratelimitDescriptionAnnotations ...*v2.RateLimitDescription,
) annotations.Annotations {
	outputAnnotations := annotations.Annotations{}
	for _, annotation := range ratelimitDescriptionAnnotations {
		outputAnnotations.Append(annotation)
	}

	return outputAnnotations
}

// shouldIncludeUser only include extant, human users.
func shouldIncludeUser(ctx context.Context, user client.ConfluenceUser) bool {
	logger := ctxzap.Extract(ctx)
	if user.AccountType != accountTypeAtlassian {
		logger.Debug("confluence: user is not of type atlassian", zap.Any("user", user))
		return false
	}
	return true
}

func confluencePrincipalType(resourceTypeId string) (string, error) {
	switch resourceTypeId {
	case "user":
		return "USER", nil
	case "group":
		return "GROUP", nil
	}
	return "", fmt.Errorf("unsupported principal resource type: %s", resourceTypeId)
}

func NewPermissionEntitlement(resource *v2.Resource, id string, name string, entitlementOptions ...entitlement.EntitlementOption) *v2.Entitlement {
	entitlement := v2.Entitlement_builder{
		Id:          entitlement.NewEntitlementID(resource, id),
		DisplayName: name,
		Slug:        name,
		Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
		Resource:    resource,
	}.Build()

	for _, entitlementOption := range entitlementOptions {
		entitlementOption(entitlement)
	}
	return entitlement
}
