package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/conductorone/baton-confluence/pkg/connector"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-confluence",
		getConnector,
		configuration,
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

var defaultNouns = []string{
	"attachment",
	"blogpost",
	"comment",
	"page",
	"space",
}

var defaultVerbs = []string{
	"administer",
	"archive",
	"create",
	"delete",
	"export",
	"read",
	"restrict_content",
	"update",
}

func filterArgs(args, defaults []string) ([]string, error) {
	var validArgs []string

	argsSet := mapset.NewSet(args...)
	defaultsSet := mapset.NewSet(defaults...)

	// First, validate that all args from the cli are valid
	for _, arg := range args {
		if !defaultsSet.Contains(arg) {
			return nil, fmt.Errorf("invalid input: %s", arg)
		}
	}

	// If there were no flags on the cli, use the defaults
	if argsSet.Cardinality() == 0 {
		return defaults, nil
	}

	// Otherwise, grab words from the defaults in the right order
	for _, arg := range defaults {
		if argsSet.Contains(arg) {
			validArgs = append(validArgs, arg)
		}
	}

	// Just double check that validArgs actually has elements
	if len(validArgs) == 0 {
		return nil, errors.New("missing valid args")
	}

	return validArgs, nil
}

func getConnector(ctx context.Context, v *viper.Viper) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)

	err := field.Validate(configuration, v)
	if err != nil {
		return nil, err
	}

	nouns, err := filterArgs(v.GetStringSlice(nounsField.FieldName), defaultNouns)
	if err != nil {
		l.Error("invalid nouns", zap.Error(err))
		return nil, err
	}

	verbs, err := filterArgs(v.GetStringSlice(verbsField.FieldName), defaultVerbs)
	if err != nil {
		l.Error("invalid verbs", zap.Error(err))
		return nil, err
	}

	cb, err := connector.New(
		ctx,
		v.GetString(apiKeyField.FieldName),
		v.GetString(domainUrl.FieldName),
		v.GetString(usernameField.FieldName),
		v.GetBool(skipPersonalSpaces.FieldName),
		nouns,
		verbs,
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
