package connector

import (
	"context"
	"errors"
	"fmt"
	"io"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/conductorone/baton-confluence/pkg/connector/client"
)

const (
	accountTypeAtlassian = "atlassian" // user account type
	accountTypeApp       = "app"       // bot account type

	resourceTypeGroupID = "group"
	resourceTypeUserID  = "user"
	resourceTypeSpaceID = "space"
)

var (
	resourceTypeGroup = &v2.ResourceType{
		Id:          resourceTypeGroupID,
		DisplayName: "Group",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	}
	resourceTypeUser = &v2.ResourceType{
		Id:          resourceTypeUserID,
		DisplayName: "User",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_USER,
		},
		Annotations: annotationsForUserResourceType(),
	}
	spaceResourceType = &v2.ResourceType{
		Id:          resourceTypeSpaceID,
		DisplayName: "Space",
		Traits:      []v2.ResourceType_Trait{},
	}
	spaceRoleResourceType = &v2.ResourceType{
		Id:          "space_role",
		DisplayName: "Space Role",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
	}
	spaceRoleAssignmentResourceType = &v2.ResourceType{
		Id:          "space_role_assignment",
		DisplayName: "Space Role Assignment",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_SCOPE_BINDING},
		Annotations: annotationsSkipEntitlements(),
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
	useRbac            bool
	nouns              []string
	verbs              []string
}

var defaultNouns = []string{
	"attachment",
	"blogpost",
	"comment",
	"page",
	resourceTypeSpaceID,
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

	// If there were no args at all then use the defaults
	if argsSet.Cardinality() == 0 {
		return defaults, nil
	}

	// Validate that all args are valid
	for _, arg := range args {
		if !defaultsSet.Contains(arg) {
			return nil, fmt.Errorf("invalid input: %s", arg)
		}
	}

	// Otherwise, grab from the defaults in the right order
	for _, arg := range defaults {
		if argsSet.Contains(arg) {
			validArgs = append(validArgs, arg)
		}
	}

	// Just double check that validArgs isn't empty
	if len(validArgs) == 0 {
		return nil, errors.New("missing valid args")
	}

	return validArgs, nil
}

func New(
	ctx context.Context,
	apiKey string,
	domainUrl string,
	username string,
	skipPersonalSpaces bool,
	useRbac bool,
	nouns []string,
	verbs []string,
) (*Confluence, error) {
	client, err := client.NewConfluenceClient(ctx, username, apiKey, domainUrl)
	if err != nil {
		return nil, err
	}

	filteredNouns, err := filterArgs(nouns, defaultNouns)
	if err != nil {
		return nil, err
	}

	filteredVerbs, err := filterArgs(verbs, defaultVerbs)
	if err != nil {
		return nil, err
	}

	rv := &Confluence{
		domain:             domainUrl,
		apiKey:             apiKey,
		userName:           username,
		client:             client,
		skipPersonalSpaces: skipPersonalSpaces,
		useRbac:            useRbac,
		nouns:              filteredNouns,
		verbs:              filteredVerbs,
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
	var err error
	if c.useRbac {
		err = c.client.VerifyRbac(ctx)
	} else {
		err = c.client.Verify(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("confluence-connector: failed to validate API keys: %w", err)
	}

	return nil, nil
}

func (c *Confluence) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

func (c *Confluence) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	syncers := []connectorbuilder.ResourceSyncer{
		groupBuilder(c.client),
		userBuilder(c.client),
		newSpaceBuilder(c.client, c.skipPersonalSpaces, c.useRbac, c.nouns, c.verbs),
	}
	if c.useRbac {
		syncers = append(syncers, newSpaceRoleBuilder(c.client))
		syncers = append(syncers, newSpaceRoleAssignmentBuilder(c.client))
	}
	return syncers
}
