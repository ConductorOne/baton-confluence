package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
)

const (
	maxResults       = 50
	minRatelimitWait = 1 * time.Second                        // Minimum time to wait after a request was ratelimited before trying again
	maxRatelimitWait = (2 * time.Minute) + (30 * time.Second) // Maximum time to wait after a request was ratelimited before erroring
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
	httpClient *http.Client
	user       string
	apiKey     string
	apiBase    *url.URL
}

func NewConfluenceClient(ctx context.Context, user, apiKey, domain string) (*ConfluenceClient, error) {
	apiBase, err := url.Parse(fmt.Sprintf("https://%s/wiki/rest/api/", domain))
	if err != nil {
		return nil, err
	}

	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, nil))
	if err != nil {
		return nil, err
	}

	return &ConfluenceClient{
		httpClient: httpClient,
		apiBase:    apiBase,
		user:       user,
		apiKey:     apiKey,
	}, nil
}

func (c *ConfluenceClient) Verify(ctx context.Context) error {
	u, err := c.genURLNonPaginated("user/current")
	if err != nil {
		return err
	}

	var resp *ConfluenceUser
	if err := c.get(ctx, u, &resp); err != nil {
		return err
	}

	currentUser := resp.AccountId
	if currentUser == "" {
		return errors.New("failed to find new user")
	}

	return nil
}

func (c *ConfluenceClient) GetUsers(ctx context.Context, pageToken string, pageSize int) ([]ConfluenceUser, string, error) {
	// There is no api to get all users, so get all groups then all members of each group.
	// We also have to internally handle paging, due to the multiple layers of requests.

	groups := make([]ConfluenceGroup, 0)

	// Get all groups
	for {
		u, err := c.genURL(pageToken, pageSize, "group")
		if err != nil {
			return nil, "", err
		}

		var resp *confluenceGroupList
		if err := c.get(ctx, u, &resp); err != nil {
			return nil, "", err
		}

		groupPage := resp.Results
		if len(groupPage) == 0 {
			break
		}
		groups = append(groups, groupPage...)
		pageToken = incToken(pageToken, len(groupPage))
	}

	if len(groups) == 0 {
		return []ConfluenceUser{}, "", nil
	}

	userMap := make(map[string]ConfluenceUser)

	// Get members of each group
	for _, group := range groups {
		users := make([]ConfluenceUser, 0)
		pageToken = ""
		for {
			u, err := c.genURL(pageToken, pageSize, "group/member?name="+group.Name)
			if err != nil {
				return nil, "", err
			}

			var resp *confluenceUserList
			if err := c.get(ctx, u, &resp); err != nil {
				return nil, "", err
			}

			userPage := resp.Results
			if len(userPage) == 0 {
				break
			}

			users = append(users, userPage...)

			pageToken = incToken(pageToken, len(userPage))
		}

		// De-dupe users accross groups
		for _, user := range users {
			if _, ok := userMap[user.AccountId]; !ok {
				userMap[user.AccountId] = user
			}
		}
	}

	allUsers := make([]ConfluenceUser, 0)
	for _, user := range userMap {
		allUsers = append(allUsers, user)
	}

	return allUsers, "", nil
}

func (c *ConfluenceClient) GetGroups(ctx context.Context, pageToken string, pageSize int) ([]ConfluenceGroup, string, error) {
	u, err := c.genURL(pageToken, pageSize, "group")
	if err != nil {
		return nil, "", err
	}

	var resp *confluenceGroupList
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, "", err
	}

	groups := resp.Results

	if len(groups) == 0 {
		return groups, "", nil
	}

	token := incToken(pageToken, len(groups))

	return groups, token, nil
}

func (c *ConfluenceClient) GetGroupMembers(ctx context.Context, pageToken string, pageSize int, group string) ([]ConfluenceUser, string, error) {
	u, err := c.genURL(pageToken, pageSize, "group/member?name="+group)
	if err != nil {
		return nil, "", err
	}

	var resp *confluenceUserList
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, "", err
	}

	users := resp.Results

	if len(users) == 0 {
		return users, "", nil
	}

	token := incToken(pageToken, len(users))

	return users, token, nil
}

func (c *ConfluenceClient) get(ctx context.Context, u *url.URL, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.user, c.apiKey)

	for {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		retryAfter := strToInt(resp.Header.Get("Retry-After"))

		switch resp.StatusCode {
		case http.StatusOK:
			if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
				return fmt.Errorf("failed to decode response body for '%s': %w", u, err)
			}
			return nil
		case http.StatusTooManyRequests:
			if err := wait(ctx, retryAfter); err != nil {
				return fmt.Errorf("confluence-connector: failed to wait for retry on '%s': %w", u, err)
			}
			continue
		case http.StatusServiceUnavailable:
			// Per the docs: transient 5XX errors should be treated as 429/too-many-requests if they have a retry header.
			// 503 errors were the only ones explicitly called out, but I guess it's possible for others too.
			// https://developer.atlassian.com/cloud/confluence/rate-limiting/
			if retryAfter != 0 {
				if err := wait(ctx, retryAfter); err != nil {
					return fmt.Errorf("confluence-connector: failed to wait for retry on '%s': %w", u, err)
				}
				continue
			}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading non-200 response body: %w", err)
		}

		return &RequestError{
			URL:    u,
			Status: resp.StatusCode,
			Body:   string(body),
		}
	}
}

func (c *ConfluenceClient) genURLNonPaginated(path string) (*url.URL, error) {
	parsed, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request path '%s': %w", path, err)
	}
	u := c.apiBase.ResolveReference(parsed)
	return u, nil
}

func (c *ConfluenceClient) genURL(pageToken string, pageSize int, path string) (*url.URL, error) {
	parsed, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request path '%s': %w", path, err)
	}

	u := c.apiBase.ResolveReference(parsed)

	max := pageSize
	if max == 0 || max > maxResults {
		max = maxResults
	}

	q := u.Query()
	q.Set("start", pageToken)
	q.Set("limit", strconv.Itoa(max))
	u.RawQuery = q.Encode()

	return u, nil
}

func wait(ctx context.Context, retryAfter int) error {
	now := time.Now()
	resetAt := now.Add(time.Duration(retryAfter) * time.Second)

	// Wait must be within min/max window
	d := resetAt.Sub(now)
	if d < minRatelimitWait {
		d = minRatelimitWait
	} else if d > maxRatelimitWait {
		d = maxRatelimitWait
	}

	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
