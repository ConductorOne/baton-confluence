package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	apiKeyField = field.StringField(
		"api-key",
		field.WithDescription("The API key for your Confluence account"),
		field.WithRequired(true),
	)
	domainUrl = field.StringField(
		"domain-url",
		field.WithDescription("The domain URL for your Confluence account"),
		field.WithRequired(true),
	)
	usernameField = field.StringField(
		"username",
		field.WithDescription("The username for your Confluence account"),
		field.WithRequired(true),
	)
	skipPersonalSpaces = field.BoolField(
		"skip-personal-spaces",
		field.WithDescription("Skip syncing personal spaces and their permissions"),
		field.WithRequired(false),
	)
        nounsField = field.StringArrayField(
		"noun",
		field.WithDescription("The nouns for your Confluence Space sync"),
		field.WithRequired(false),
	)
        verbsField = field.StringArrayField(
		"verb",
		field.WithDescription("The verbs for your Confluence Space sync"),
		field.WithRequired(false),
	)
)

var configuration = field.NewConfiguration(
	[]field.SchemaField{
		apiKeyField,
		domainUrl,
		usernameField,
		skipPersonalSpaces,
		nounsField,
		verbsField,
	},
)
