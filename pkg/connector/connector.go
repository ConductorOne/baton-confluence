package connector

import (
	"context"
	"fmt"
	"io"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
)

const (
	accountTypeAtlassian = "atlassian" // user account type
	accountTypeApp       = "app"       // bot account type
)

var (
	resourceTypeGroup = &v2.ResourceType{
		Id:          "group",
		DisplayName: "Group",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	}
	resourceTypeUser = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_USER,
		},
		Annotations: annotationsForUserResourceType(),
	}
	spaceResourceType = &v2.ResourceType{
		Id:          "space",
		DisplayName: "Space",
		Traits:      []v2.ResourceType_Trait{},
	}
)

type Config struct {
	UserName string
	ApiKey   string
	Domain   string
}

type Confluence struct {
	client             *client.ConfluenceClient
	domain             string
	apiKey             string
	userName           string
	skipPersonalSpaces bool
	nouns              []string
	verbs              []string
}

func New(
	ctx context.Context,
	apiKey string,
	domainUrl string,
	username string,
	skipPersonalSpaces bool,
	nouns []string,
	verbs []string,
) (*Confluence, error) {
	client, err := client.NewConfluenceClient(ctx, username, apiKey, domainUrl)
	if err != nil {
		return nil, err
	}
	rv := &Confluence{
		domain:             domainUrl,
		apiKey:             apiKey,
		userName:           username,
		client:             client,
		skipPersonalSpaces: skipPersonalSpaces,
		nouns:              nouns,
		verbs:              verbs,
	}
	return rv, nil
}

func (c *Confluence) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	var annos annotations.Annotations
	annos.Update(&v2.ExternalLink{
		Url: c.domain,
	})

	return &v2.ConnectorMetadata{
		DisplayName: "Confluence",
		Description: "Connector syncing Confluence users and groups to Baton",
		Annotations: annos,
	}, nil
}

func (c *Confluence) Validate(ctx context.Context) (annotations.Annotations, error) {
	err := c.client.Verify(ctx)
	if err != nil {
		return nil, fmt.Errorf("confluence-connector: failed to validate API keys: %w", err)
	}

	return nil, nil
}

func (c *Confluence) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

func (c *Confluence) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		groupBuilder(c.client),
		userBuilder(c.client),
		newSpaceBuilder(c.client, c.skipPersonalSpaces, c.nouns, c.verbs),
	}
}
