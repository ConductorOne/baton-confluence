package client

import (
	"net/http"
	"slices"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/helpers"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
)

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
