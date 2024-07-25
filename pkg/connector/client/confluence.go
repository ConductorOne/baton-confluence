package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/helpers"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxResults = 50
)

type RequestError struct {
	Status int
	URL    *url.URL
	Body   string
}

func (r *RequestError) Error() string {
	return fmt.Sprintf("confluence-connector: request error. Status: %d, Url: %s, Body: %s", r.Status, r.URL, r.Body)
}

type ConfluenceClient struct {
	user    string
	apiKey  string
	apiBase *url.URL
	wrapper *uhttp.BaseHttpClient
}

// fallBackToHTTPS checks to domain and tacks on "https://" if no scheme is
// specified. This exists so that a user can override the scheme by including it
// in the passed "domain-url" config.
func fallBackToHTTPS(domain string) (*url.URL, error) {
	parsed, err := url.Parse(domain)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" {
		parsed, err = url.Parse(fmt.Sprintf("https://%s", domain))
		if err != nil {
			return nil, err
		}
	}
	return parsed, nil
}

func NewConfluenceClient(ctx context.Context, user, apiKey, domain string) (*ConfluenceClient, error) {
	apiBase, err := fallBackToHTTPS(domain)
	if err != nil {
		return nil, err
	}

	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, nil))
	if err != nil {
		return nil, err
	}

	return &ConfluenceClient{
		apiBase: apiBase,
		apiKey:  apiKey,
		user:    user,
		wrapper: uhttp.NewBaseHttpClient(httpClient),
	}, nil
}

func (c *ConfluenceClient) Verify(ctx context.Context) error {
	currentUserUrl, err := c.genURLNonPaginated(CurrentUserUrlPath)
	if err != nil {
		return err
	}

	var response *ConfluenceUser
	_, err = c.get(ctx, currentUserUrl, &response)
	if err != nil {
		return err
	}

	currentUser := response.AccountId
	if currentUser == "" {
		return errors.New("failed to find new user")
	}

	return nil
}

func isThereAnotherPage(links ConfluenceLink) bool {
	return links.Next != ""
}

func (c *ConfluenceClient) GetGroups(
	ctx context.Context,
	pageToken string,
	pageSize int,
) (
	[]ConfluenceGroup,
	string,
	*v2.RateLimitDescription,
	error,
) {
	groupsUrl, err := c.genURL(pageToken, pageSize, GroupsListUrlPath)
	if err != nil {
		return nil, "", nil, err
	}

	var response *confluenceGroupList
	ratelimitData, err := c.get(ctx, groupsUrl, &response)
	if err != nil {
		return nil, "", ratelimitData, err
	}

	groups := response.Results

	if !isThereAnotherPage(response.Links) {
		return groups, "", ratelimitData, nil
	}

	token := incToken(pageToken, len(groups))

	return groups, token, ratelimitData, nil
}

func (c *ConfluenceClient) GetGroupMembers(
	ctx context.Context,
	pageToken string,
	pageSize int,
	groupId string,
) (
	[]ConfluenceUser,
	string,
	*v2.RateLimitDescription,
	error,
) {
	getUsersUrl, err := c.parse(
		fmt.Sprintf(getUsersByGroupIdUrlPath, groupId),
		withLimitAndOffset(pageToken, pageSize),
		withQueryParameters(map[string]interface{}{
			"expand": "operations",
		}),
	)
	if err != nil {
		return nil, "", nil, err
	}

	var response *confluenceUserList
	ratelimitData, err := c.get(ctx, getUsersUrl, &response)
	if err != nil {
		return nil, "", ratelimitData, err
	}

	users := response.Results

	if !isThereAnotherPage(response.Links) {
		return users, "", ratelimitData, nil
	}

	token := incToken(pageToken, len(users))

	return users, token, ratelimitData, nil
}

func (c *ConfluenceClient) AddUserToGroup(
	ctx context.Context,
	accountID string,
	groupId string,
) (*v2.RateLimitDescription, error) {
	getUsersUrl, err := c.genURLNonPaginated(
		fmt.Sprintf(
			addUsersToGroupUrlPath,
			groupId,
		),
	)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(
		AddUserToGroupRequestBody{
			AccountId: accountID,
		},
	)
	if err != nil {
		return nil, err
	}

	body := strings.NewReader(string(bodyBytes))
	ratelimitData, err := c.post(ctx, getUsersUrl, nil, body)
	if err != nil {
		return ratelimitData, err
	}
	return ratelimitData, nil
}

func (c *ConfluenceClient) RemoveUserFromGroup(
	ctx context.Context,
	accountID string,
	groupId string,
) (*v2.RateLimitDescription, error) {
	getUsersUrl, err := c.genURLNonPaginated(
		fmt.Sprintf(
			removeUsersFromGroupUrlPath,
			groupId,
			accountID,
		),
	)
	if err != nil {
		return nil, err
	}

	ratelimitData, err := c.delete(ctx, getUsersUrl, nil)
	if err != nil {
		return ratelimitData, err
	}
	return ratelimitData, nil
}

func (c *ConfluenceClient) get(
	ctx context.Context,
	getUrl *url.URL,
	target interface{},
) (*v2.RateLimitDescription, error) {
	return c.makeRequest(ctx, getUrl, target, http.MethodGet, nil)
}

func (c *ConfluenceClient) post(
	ctx context.Context,
	postUrl *url.URL,
	target interface{},
	requestBody io.Reader,
) (*v2.RateLimitDescription, error) {
	return c.makeRequest(ctx, postUrl, target, http.MethodPost, requestBody)
}

func (c *ConfluenceClient) delete(
	ctx context.Context,
	deleteUrl *url.URL,
	target interface{},
) (*v2.RateLimitDescription, error) {
	return c.makeRequest(ctx, deleteUrl, target, http.MethodDelete, nil)
}

func (c *ConfluenceClient) makeRequest(
	ctx context.Context,
	url *url.URL,
	target interface{},
	method string,
	requestBody io.Reader,
) (*v2.RateLimitDescription, error) {
	req, err := http.NewRequestWithContext(ctx, method, url.String(), requestBody)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.user, c.apiKey)

	ratelimitData := v2.RateLimitDescription{}

	response, err := c.wrapper.Do(
		req,
		WithConfluenceRatelimitData(&ratelimitData),
		uhttp.WithJSONResponse(target),
	)
	if err == nil {
		return &ratelimitData, nil
	}
	if response == nil {
		return nil, err
	}
	defer response.Body.Close()

	// If we get ratelimit data back (e.g. the "Retry-After" header) or a
	// "ratelimit-like" status code, then return a recoverable gRPC code.
	if isRatelimited(ratelimitData.Status, response.StatusCode) {
		return &ratelimitData, status.Error(codes.Unavailable, response.Status)
	}

	// If it's some other error, it is unrecoverable.
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return nil, &RequestError{
		URL:    url,
		Status: response.StatusCode,
		Body:   string(responseBody),
	}
}

// genURLNonPaginated adds the given URL path to the API base URL.
func (c *ConfluenceClient) genURLNonPaginated(path string) (*url.URL, error) {
	parsed, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request path '%s': %w", path, err)
	}
	parsedUrl := c.apiBase.ResolveReference(parsed)
	return parsedUrl, nil
}

// genURL adds `start` and `limit` query parameters to a URL. This pagination
// parameter is only used by the v1 REST API.
func (c *ConfluenceClient) genURL(pageToken string, pageSize int, path string) (*url.URL, error) {
	parsed, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request path '%s': %w", path, err)
	}

	parsedUrl := c.apiBase.ResolveReference(parsed)

	maximum := pageSize
	if maximum == 0 || maximum > maxResults {
		maximum = maxResults
	}

	query := parsedUrl.Query()
	query.Set("start", pageToken)
	query.Set("limit", strconv.Itoa(maximum))
	parsedUrl.RawQuery = query.Encode()

	return parsedUrl, nil
}

func incToken(pageToken string, count int) string {
	token := strToInt(pageToken)

	token += count
	if token == 0 {
		return ""
	}

	return strconv.Itoa(token)
}

func strToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

// GetSpaces uses pagination to get a list of spaces from the global list.
func (c *ConfluenceClient) GetSpaces(
	ctx context.Context,
	pageSize int,
	paginationCursor string,
) (
	[]ConfluenceSpace,
	string,
	*v2.RateLimitDescription,
	error,
) {
	spacesListUrl, err := c.genURLWithPaginationCursor(
		SpacesListUrlPath,
		pageSize,
		paginationCursor,
	)
	if err != nil {
		return nil, "", nil, err
	}

	var response *confluenceSpaceList
	ratelimitData, err := c.get(ctx, spacesListUrl, &response)
	if err != nil {
		return nil, "", ratelimitData, err
	}

	cursor := extractPaginationCursor(response.Links)
	spaces := response.Results

	return spaces, cursor, ratelimitData, nil
}

func (c *ConfluenceClient) ConfluenceSpaceOperations(
	ctx context.Context,
	cursor string,
	pageSize int,
	spaceId string,
) (
	[]ConfluenceSpaceOperation,
	string,
	*v2.RateLimitDescription,
	error,
) {
	logger := ctxzap.Extract(ctx)
	logger.Debug("fetching space", zap.String("spaceId", spaceId))

	spaceUrl, err := c.genURLWithPaginationCursor(
		fmt.Sprintf(spacesGetUrlPath+"?include-operations=1", spaceId),
		pageSize,
		cursor,
	)

	if err != nil {
		return nil, "", nil, err
	}

	var response *ConfluenceSpace
	ratelimitData, err := c.get(ctx, spaceUrl, &response)
	if err != nil {
		return nil, "", ratelimitData, err
	}

	operations := make([]ConfluenceSpaceOperation, 0)
	operations = append(operations, response.Operations.Results...)

	nextToken := ""
	if response.Operations.Meta.HasMore {
		nextToken = response.Operations.Meta.Cursor
	}

	return operations, nextToken, ratelimitData, nil
}

func (c *ConfluenceClient) GetSpacePermissions(
	ctx context.Context,
	pageToken string,
	pageSize int,
	spaceId string,
) (
	[]ConfluenceSpacePermission,
	string,
	*v2.RateLimitDescription,
	error,
) {
	spacePermissionsListUrl, err := c.genURLWithPaginationCursor(
		fmt.Sprintf(SpacePermissionsListUrlPath, spaceId),
		pageSize,
		pageToken,
	)
	if err != nil {
		return nil, "", nil, err
	}

	var response *ConfluenceSpacePermissionResponse
	ratelimitData, err := c.get(
		ctx,
		spacePermissionsListUrl,
		&response,
	)
	if err != nil {
		return nil, "", ratelimitData, err
	}
	cursor := extractPaginationCursor(response.Links)
	permissions := make([]ConfluenceSpacePermission, 0)
	permissions = append(permissions, response.Results...)

	return permissions, cursor, ratelimitData, nil
}

// getSubjectTypeFromPrincipalType map between ConductorOne representation and
// Confluence representation. It just so happens that the representations are
// the same, but I don't want to pass it straight along in case we get new
// principal types that aren't a 100% match.
func getSubjectTypeFromPrincipalType(principalType string) (string, error) {
	switch principalType {
	case "user":
		return "user", nil
	case "group":
		return "group", nil
	}
	return "", fmt.Errorf("principal type '%s' is not supported", principalType)
}

func (c *ConfluenceClient) AddSpacePermission(
	ctx context.Context,
	spaceName string,
	key string,
	target string,
	principalId string,
	principalType string,
) (
	*v2.RateLimitDescription,
	error,
) {
	spacePermissionsListUrl, err := c.genURLNonPaginated(
		fmt.Sprintf(spacePermissionsCreateUrlPath, spaceName),
	)
	if err != nil {
		return nil, err
	}

	subjectType, err := getSubjectTypeFromPrincipalType(principalType)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(
		CreateSpacePermissionRequestBody{
			SpacePermissionSubject{
				Identifier: principalId,
				Type:       subjectType,
			},
			SpacePermissionOperation{
				Key:    key,
				Target: target,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	body := strings.NewReader(string(bodyBytes))

	var response bool
	ratelimitData, err := c.post(
		ctx,
		spacePermissionsListUrl,
		&response,
		body,
	)
	if err != nil {
		return ratelimitData, err
	}

	return ratelimitData, nil
}

// findSpacePermission - There isn't a way to look up a permission by these
// fields, so we need to list _all_ permissions in order to find the permission.
func (c *ConfluenceClient) findSpacePermission(
	ctx context.Context,
	spaceId string,
	key string,
	target string,
	principalId string,
	principalType string,
) (
	*ConfluenceSpacePermission,
	*v2.RateLimitDescription,
	error,
) {
	// We need to list _all_ permissions in order to figure out the permission's ID.
	cursor := ""
	for {
		listPermissionsUrl, err := c.genURLWithPaginationCursor(
			fmt.Sprintf(
				SpacePermissionsListUrlPath,
				spaceId,
			),
			maxResults,
			cursor,
		)
		if err != nil {
			return nil, nil, err
		}

		var response *ConfluenceSpacePermissionResponse
		ratelimitData, err := c.get(
			ctx,
			listPermissionsUrl,
			&response,
		)
		if err != nil {
			return nil, ratelimitData, err
		}
		for _, permission := range response.Results {
			if permission.Principal.Id == principalId &&
				permission.Principal.Type == principalType &&
				permission.Operation.Key == key &&
				permission.Operation.TargetType == target {
				return &permission, ratelimitData, nil
			}
		}
		cursor = extractPaginationCursor(response.Links)
		if cursor == "" {
			break
		}
	}

	return nil, nil, fmt.Errorf("space permission not found")
}

// findSpace - The v1 and v2 API are slightly different. The former uses "space
// key", which is like the URL slug for the space. The latter use plain ID.
func (c *ConfluenceClient) findSpace(
	ctx context.Context,
	spaceId string,
) (
	*ConfluenceSpace,
	*v2.RateLimitDescription,
	error,
) {
	getSpaceUrl, err := c.genURLNonPaginated(
		fmt.Sprintf(
			spacesGetUrlPath,
			spaceId,
		),
	)
	if err != nil {
		return nil, nil, err
	}

	var response *ConfluenceSpace
	ratelimitData, err := c.get(
		ctx,
		getSpaceUrl,
		&response,
	)
	if err != nil {
		return nil, ratelimitData, err
	}
	return response, ratelimitData, nil
}

func (c *ConfluenceClient) RemoveSpacePermission(
	ctx context.Context,
	spaceId string,
	key string,
	target string,
	principalId string,
	principalType string,
) (
	*v2.RateLimitDescription,
	error,
) {
	permission, ratelimitData, err := c.findSpacePermission(
		ctx,
		spaceId,
		key,
		target,
		principalId,
		principalType,
	)

	if err != nil {
		return ratelimitData, err
	}

	space, ratelimitData, err := c.findSpace(ctx, spaceId)
	if err != nil {
		return ratelimitData, err
	}

	deletePermissionUrl, err := c.genURLNonPaginated(
		fmt.Sprintf(
			spacePermissionsUpdateUrlPath,
			space.Key,
			permission.Id,
		),
	)
	if err != nil {
		return nil, err
	}

	var response bool
	ratelimitData, err = c.delete(
		ctx,
		deletePermissionUrl,
		&response,
	)
	if err != nil {
		return ratelimitData, err
	}

	return ratelimitData, nil
}

// extractPaginationCursor returns the query parameters from the "next" link in
// the list response.
func extractPaginationCursor(links ConfluenceLink) string {
	parsedUrl, err := url.Parse(links.Next)
	if err != nil {
		return ""
	}
	return parsedUrl.Query().Get("cursor")
}

// WithConfluenceRatelimitData Per the docs: transient 5XX errors should be
// treated as 429/too-many-requests if they have a retry header. 503 errors were
// the only ones explicitly called out, but I guess it's possible for others too
// https://developer.atlassian.com/cloud/confluence/rate-limiting/
func WithConfluenceRatelimitData(resource *v2.RateLimitDescription) uhttp.DoOption {
	return func(response *uhttp.WrapperResponse) error {
		rateLimitData, err := helpers.ExtractRateLimitData(response.StatusCode, &response.Header)
		if err != nil {
			return err
		}
		resource = rateLimitData
		return nil
	}
}

func isRatelimited(
	ratelimitStatus v2.RateLimitDescription_Status,
	statusCode int,
) bool {
	return slices.Contains(
		[]v2.RateLimitDescription_Status{
			v2.RateLimitDescription_STATUS_OVERLIMIT,
			v2.RateLimitDescription_STATUS_ERROR,
		},
		ratelimitStatus,
	) || slices.Contains(
		[]int{
			http.StatusTooManyRequests,
			http.StatusGatewayTimeout,
			http.StatusServiceUnavailable,
		},
		statusCode,
	)
}

// GetUsersFromSearch There are no official, documented ways to get lists of
// users in Confluence. One way to get users is to issue a CQL search query with
// no conditions. The documentation mentions that queries return "up to 10k"
// users. So that may end up being a limitation of this approach.
func (c *ConfluenceClient) GetUsersFromSearch(
	ctx context.Context,
	pageToken string,
	pageSize int,
) (
	[]ConfluenceUser,
	string,
	*v2.RateLimitDescription,
	error,
) {
	getUsersUrl, err := c.parse(
		SearchUrlPath,
		withLimitAndOffset(pageToken, pageSize),
		withQueryParameters(map[string]interface{}{
			"cql":    "type=user",
			"expand": "operations",
		}),
	)
	if err != nil {
		return nil, "", nil, err
	}

	var response *ConfluenceSearchList
	ratelimitData, err := c.get(ctx, getUsersUrl, &response)
	if err != nil {
		return nil, "", ratelimitData, err
	}

	users := make([]ConfluenceUser, 0)
	for _, user := range response.Results {
		users = append(users, user.User)
	}

	// The only way we can tell that we've hit the end of the list is if we get
	// back fewer results than we asked for. If we get the last page but there
	// are `pageSize`, then `.List()` still has to fetch the blank next page.
	if len(users) < pageSize {
		return users, "", ratelimitData, nil
	}

	token := incToken(pageToken, len(users))
	return users, token, ratelimitData, nil
}
