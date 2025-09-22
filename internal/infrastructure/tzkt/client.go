package tzkt

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	resty "github.com/go-resty/resty/v2"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/metrics"
	"golang.org/x/time/rate"
)

type Client struct {
	baseURL     string
	httpClient  *resty.Client
	logger      *logger.Logger
	rateLimiter *rate.Limiter
	maxRetries  int
	retryDelay  time.Duration
}

func NewClient(baseURL string, timeout time.Duration, maxRetries int, retryDelay time.Duration, log *logger.Logger) *Client {
	httpClient := resty.New().
		SetTimeout(timeout).
		SetRetryCount(maxRetries).
		SetRetryWaitTime(retryDelay).
		SetRetryMaxWaitTime(retryDelay * 3).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			return err != nil || r.StatusCode() >= 500 || r.StatusCode() == 429
		})

	return &Client{
		baseURL:     baseURL,
		httpClient:  httpClient,
		logger:      log,
		rateLimiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 10),
		maxRetries:  maxRetries,
		retryDelay:  retryDelay,
	}
}

func (c *Client) GetDelegations(ctx context.Context, params QueryParams) ([]DelegationResponse, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	queryParams := c.buildQueryParams(params)

	url := fmt.Sprintf("%s/v1/operations/delegations", c.baseURL)

	c.logger.Debugw("Fetching delegations", "url", url, "params", queryParams)

	start := time.Now()
	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetQueryParams(queryParams).
		SetHeader("Accept", "application/json").
		Get(url)

	duration := time.Since(start).Seconds()
	success := err == nil && resp.StatusCode() == 200
	metrics.RecordTzktAPIRequest(duration, success)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch delegations: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), string(resp.Body()))
	}

	var delegations []DelegationResponse
	if err := json.Unmarshal(resp.Body(), &delegations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.logger.Debugw("Fetched delegations", "count", len(delegations))

	return delegations, nil
}

func (c *Client) GetDelegationsSince(ctx context.Context, timestamp time.Time, limit int) ([]DelegationResponse, error) {
	params := QueryParams{
		Limit: limit,
		Timestamp: &TimestampFilter{
			Gte: &timestamp,
		},
		Sort: []string{"id.asc"},
	}

	return c.GetDelegations(ctx, params)
}

func (c *Client) GetDelegationsFromLevel(ctx context.Context, level int64, limit int) ([]DelegationResponse, error) {
	params := QueryParams{
		Limit: limit,
		Level: &LevelFilter{
			Gte: &level,
		},
		Sort: []string{"id.asc"},
	}

	return c.GetDelegations(ctx, params)
}

func (c *Client) GetHistoricalDelegations(ctx context.Context, startDate time.Time, batchSize int) (<-chan []DelegationResponse, <-chan error) {
	delegationsChan := make(chan []DelegationResponse, 10)
	errorChan := make(chan error, 1)

	go func() {
		defer close(delegationsChan)
		defer close(errorChan)

		offset := 0
		for {
			select {
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			default:
			}

			params := QueryParams{
				Limit:  batchSize,
				Offset: offset,
				Timestamp: &TimestampFilter{
					Gte: &startDate,
				},
				Sort: []string{"id.asc"},
			}

			delegations, err := c.GetDelegations(ctx, params)
			if err != nil {
				errorChan <- err
				return
			}

			if len(delegations) == 0 {
				// No more data available
				return
			}

			delegationsChan <- delegations
			offset += len(delegations)

			// Continue fetching - only stop when we get 0 delegations
		}
	}()

	return delegationsChan, errorChan
}

func (c *Client) buildQueryParams(params QueryParams) map[string]string {
	queryParams := make(map[string]string)

	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	if params.Offset > 0 {
		queryParams["offset"] = strconv.Itoa(params.Offset)
	}

	if params.Level != nil {
		if params.Level.Gte != nil {
			queryParams["level.ge"] = strconv.FormatInt(*params.Level.Gte, 10)
		}
		if params.Level.Gt != nil {
			queryParams["level.gt"] = strconv.FormatInt(*params.Level.Gt, 10)
		}
		if params.Level.Lte != nil {
			queryParams["level.le"] = strconv.FormatInt(*params.Level.Lte, 10)
		}
		if params.Level.Lt != nil {
			queryParams["level.lt"] = strconv.FormatInt(*params.Level.Lt, 10)
		}
		if params.Level.Eq != nil {
			queryParams["level.eq"] = strconv.FormatInt(*params.Level.Eq, 10)
		}
	}

	if params.Timestamp != nil {
		if params.Timestamp.Gte != nil {
			queryParams["timestamp.ge"] = params.Timestamp.Gte.Format(time.RFC3339)
		}
		if params.Timestamp.Gt != nil {
			queryParams["timestamp.gt"] = params.Timestamp.Gt.Format(time.RFC3339)
		}
		if params.Timestamp.Lte != nil {
			queryParams["timestamp.le"] = params.Timestamp.Lte.Format(time.RFC3339)
		}
		if params.Timestamp.Lt != nil {
			queryParams["timestamp.lt"] = params.Timestamp.Lt.Format(time.RFC3339)
		}
	}

	if len(params.Sort) > 0 {
		for _, s := range params.Sort {
			// Parse the sort parameter (e.g., "id.asc" or "timestamp.desc")
			parts := strings.Split(s, ".")
			if len(parts) == 2 {
				field := parts[0]
				direction := parts[1]
				// TzKT expects sort.asc=field or sort.desc=field format
				queryParams["sort."+direction] = field
			}
		}
	}

	if len(params.Select) > 0 {
		selectStr := ""
		for i, s := range params.Select {
			if i > 0 {
				selectStr += ","
			}
			selectStr += s
		}
		queryParams["select"] = selectStr
	}

	queryParams["status"] = "applied"

	return queryParams
}
