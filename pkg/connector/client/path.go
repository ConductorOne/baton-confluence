package client

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	CurrentUserUrlPath            = "/wiki/rest/api/user/current"
	GroupsListUrlPath             = "/wiki/rest/api/group"
	getUsersByGroupIdUrlPath      = "/wiki/rest/api/group/%s/membersByGroupId"
	groupBaseUrlPath              = "/wiki/rest/api/group/userByGroupId"
	SearchUrlPath                 = "/wiki/rest/api/search/user"
	spacePermissionsCreateUrlPath = "/wiki/rest/api/space/%s/permissions"
	spacePermissionsUpdateUrlPath = "/wiki/rest/api/space/%s/permissions/%s"
	SpacesListUrlPath             = "/wiki/api/v2/spaces"
	spacesGetUrlPath              = "/wiki/api/v2/spaces/%s"
	SpacePermissionsListUrlPath   = "/wiki/api/v2/spaces/%s/permissions"
)

type Option = func(*url.URL) (*url.URL, error)

func withQueryParameters(parameters map[string]interface{}) Option {
	return func(url *url.URL) (*url.URL, error) {
		query := url.Query()
		for key, interfaceValue := range parameters {
			var stringValue string
			switch actualValue := interfaceValue.(type) {
			case string:
				stringValue = actualValue
			case int:
				stringValue = strconv.Itoa(actualValue)
			case bool:
				if actualValue {
					stringValue = "1"
				} else {
					stringValue = "0"
				}
			default:
				return nil, fmt.Errorf("invalid query parameter type %s", actualValue)
			}
			query.Set(key, stringValue)
		}
		url.RawQuery = query.Encode()
		return url, nil
	}
}

// withLimitAndOffset adds `start` and `limit` query parameters to a URL. This
// pagination parameter is only used by the v1 REST API.
func withLimitAndOffset(pageToken string, pageSize int) Option {
	return withQueryParameters(map[string]interface{}{
		"limit": pageSize,
		"start": pageToken,
	})
}

// withPaginationCursor uses Confluence Cloud's REST API v2 pagination scheme.
func withPaginationCursor(pageSize int,
	paginationCursor string,
) Option {
	if pageSize < 1 {
		pageSize = 1
	}
	parameters := map[string]interface{}{
		"limit": pageSize,
	}
	if paginationCursor != "" {
		parameters["cursor"] = paginationCursor
	}

	return withQueryParameters(parameters)
}

func (c *ConfluenceClient) parse(
	path string,
	options ...Option,
) (*url.URL, error) {
	parsed, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request path '%s': %w", path, err)
	}
	parsedUrl := c.apiBase.ResolveReference(parsed)
	for _, option := range options {
		parsedUrl, err = option(parsedUrl)
		if err != nil {
			return nil, err
		}
	}
	return parsedUrl, nil
}
