package client

import (
	"context"
	"io"
	"net/http"
	"net/url"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.Header.Set("Content-Type", "application/json")

	ratelimitData := v2.RateLimitDescription{}

	doOpts := []uhttp.DoOption{
		WithConfluenceRatelimitData(&ratelimitData),
	}
	if target != nil {
		doOpts = append(doOpts, uhttp.WithJSONResponse(target))
	}
	// A response type that reports its own pagination data is additionally checked for it,
	// so a page arriving without _links fails here instead of ending the sync.
	if paginated, ok := target.(uhttp.PaginatedResponse); ok {
		doOpts = append(doOpts, uhttp.WithPaginationData(paginated))
	}

	response, err := c.wrapper.Do(
		req,
		doOpts...,
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

	// A success status that still carries an error means a DoOption failed - a decode, or
	// the pagination check below - not the request. Falling through would report a
	// RequestError with status 200 and drop the cause from the error chain.
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return &ratelimitData, err
	}

	// If it's some other error, it is unrecoverable.
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return nil, &RequestError{
		URL:    url,
		Status: response.StatusCode,
		Body:   logBody(responseBody, 2048),
	}
}

func logBody(body []byte, size int) string {
	if len(body) > size {
		return string(body[:size]) + " ..."
	}
	return string(body)
}
