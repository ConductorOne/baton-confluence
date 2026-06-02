package main

import (
	"context"
	"fmt"

	cfg "github.com/conductorone/baton-confluence/pkg/config"
	"github.com/conductorone/baton-confluence/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/cli"
	sdkConfig "github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/connectorrunner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var version = "dev"

func main() {
	ctx := context.Background()
	sdkConfig.RunConnector(
		ctx,
		"baton-confluence",
		version,
		cfg.Configuration,
		getConnector,
		connectorrunner.WithDefaultCapabilitiesConnectorBuilderV2(&connector.Confluence{}),
	)
}

func getConnector(ctx context.Context, cc *cfg.Confluence, connectorOpts *cli.ConnectorOpts) (connectorbuilder.ConnectorBuilderV2, []connectorbuilder.Opt, error) {
	if connectorOpts.SyncFilterIsExplicit() {
		willSyncRbacTypes := connectorOpts.WillSyncResourceType(connector.SpaceRoleResourceTypeID) ||
			connectorOpts.WillSyncResourceType(connector.SpaceRoleAssignmentResourceTypeID)
		if willSyncRbacTypes && !cc.UseRbac {
			return nil, nil, status.Error(codes.InvalidArgument, fmt.Sprintf("confluence-connector: use-rbac must be enabled when syncing %s or %s resource types",
				connector.SpaceRoleResourceTypeID, connector.SpaceRoleAssignmentResourceTypeID))
		}
	}

	cb, err := connector.New(
		ctx,
		cc.ApiKey,
		cc.DomainUrl,
		cc.Username,
		cc.SkipPersonalSpaces,
		cc.UseRbac,
		cc.Noun,
		cc.Verb,
	)
	if err != nil {
		return nil, nil, err
	}
	return cb, nil, nil
}
