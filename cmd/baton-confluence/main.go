package main

import (
	"context"
	"fmt"
	"os"

	cfg "github.com/conductorone/baton-confluence/pkg/config"
	"github.com/conductorone/baton-confluence/pkg/connector"
	sdkConfig "github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := sdkConfig.DefineConfiguration(
		ctx,
		"baton-confluence",
		getConnector,
		cfg.Configuration,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	cmd.Version = version

	err = cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getConnector(ctx context.Context, config *cfg.Confluence) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)

	if err := field.Validate(cfg.Configuration, config); err != nil {
		return nil, err
	}

	cb, err := connector.New(
		ctx,
		config.ApiKey,
		config.DomainUrl,
		config.Username,
		config.SkipPersonalSpaces,
		config.Noun,
		config.Verb,
	)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}

	connector, err := connectorbuilder.NewConnector(ctx, cb)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}
	return connector, nil
}
