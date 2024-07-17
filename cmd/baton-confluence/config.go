package main

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

var (
	apiKeyField = field.StringField(
		"api-key",
		field.WithDescription("The api key for your Confluence account. ($BATON_API_KEY)"),
	)
	domainUrl = field.StringField(
		"domain-url",
		field.WithDescription("The domain url for your Confluence account. ($BATON_DOMAIN_URL)"),
	)
	usernameField = field.StringField(
		"username",
		field.WithDescription("The username for your Confluence account. ($BATON_USERNAME)"),
	)
)

// configurationFields defines the external configuration required for the connector to run.
var configurationFields = []field.SchemaField{
	apiKeyField,
	domainUrl,
	usernameField,
}

// validateConfig is run after the configuration is loaded, and should return an error if it isn't valid.
func validateConfig(ctx context.Context, v *viper.Viper) error {
	if v.GetString(apiKeyField.FieldName) == "" {
		return fmt.Errorf("api key is missing")
	}
	if v.GetString(domainUrl.FieldName) == "" {
		return fmt.Errorf("domain url is missing")
	}
	if v.GetString(usernameField.FieldName) == "" {
		return fmt.Errorf("username is missing")
	}
	return nil
}
