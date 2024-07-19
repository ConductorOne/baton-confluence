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
	maxResults                    = 50
	currentUserUrlPath            = "/wiki/rest/api/user/current"
	GroupsListUrlPath             = "/wiki/rest/api/group"
	getUsersByGroupIdUrlPath      = "/wiki/rest/api/group/%s/membersByGroupId"
	addUsersToGroupUrlPath        = "/wiki/rest/api/group/userByGroupId?groupId=%s"
	removeUsersFromGroupUrlPath   = "/wiki/rest/api/group/userByGroupId?groupId=%s&accountId=%s"
	spacePermissionsCreateUrlPath = "/wiki/rest/api/space/%s/permissions"
	spacePermissionsUpdateUrlPath = "/wiki/rest/api/space/%s/permissions/%s"
	SpacesListUrlPath             = "/wiki/api/v2/spaces"
	spacesGetUrlPath              = "/wiki/api/v2/spaces/%s"
	SpacePermissionsListUrlPath   = "/wiki/api/v2/spaces/%s/permissions"
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
	currentUserUrl, err := c.genURLNonPaginated(currentUserUrlPath)
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
	getUsersUrl, err := c.genURL(pageToken, pageSize, fmt.Sprintf(getUsersByGroupIdUrlPath, groupId))
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

// genURLWithPaginationCursor uses Confluence Cloud's REST API v2 pagination scheme.
func (c *ConfluenceClient) genURLWithPaginationCursor(
	path string,
	pageSize int,
	paginationCursor string,
) (*url.URL, error) {
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
	if paginationCursor != "" {
		query.Set("cursor", paginationCursor)
	}
	query.Set("limit", strconv.Itoa(maximum))
	parsedUrl.RawQuery = query.Encode()

	return parsedUrl, nil
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
