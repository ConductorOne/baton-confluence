package connector

import (
	"context"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const ResourcesPageSize = 100

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
