//go:generate go run ./gen
package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	apiKeyField = field.StringField(
		"api-key",
		field.WithDescription("The API key for your Confluence account"),
		field.WithDisplayName("API Key"),
		field.WithRequired(true),
		field.WithIsSecret(true),
	)
	domainUrl = field.StringField(
		"domain-url",
		field.WithDescription("The domain URL for your Confluence account"),
		field.WithDisplayName("Domain URL"),
		field.WithPlaceholder("https://example.atlassian.net"),
		field.WithRequired(true),
	)
	usernameField = field.StringField(
		"username",
		field.WithDescription("The username for your Confluence account"),
		field.WithDisplayName("Username"),
		field.WithPlaceholder("user@example.com"),
		field.WithRequired(true),
	)
	skipPersonalSpaces = field.BoolField(
		"skip-personal-spaces",
		field.WithDescription("Skip syncing personal spaces and their permissions"),
		field.WithDisplayName("Skip Personal Spaces"),
		field.WithRequired(false),
	)
	nounsField = field.StringSliceField(
		"noun",
		field.WithDescription("The nouns for your Confluence Space sync"),
		field.WithDisplayName("Nouns"),
		field.WithRequired(false),
	)
	verbsField = field.StringSliceField(
		"verb",
		field.WithDescription("The verbs for your Confluence Space sync"),
		field.WithDisplayName("Verbs"),
		field.WithRequired(false),
	)
)

var ConfigurationFields = []field.SchemaField{
	apiKeyField,
	domainUrl,
	usernameField,
	skipPersonalSpaces,
	nounsField,
	verbsField,
}

var Configuration = field.NewConfiguration(
	ConfigurationFields,
	field.WithConnectorDisplayName("Confluence"),
	field.WithHelpUrl("/docs/baton/confluence"),
	field.WithIconUrl("/static/app-icons/confluence.svg"),
)
