package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// The limits of a run against the remote server.
//
// The batch API of PocketBase has a limit of its own, and the server
// refuses a batch that is too long. The number is well under the
// limit of the server, so that a run does not fail on the last
// batch.
const (
	// BatchSize is the number of records in one batch.
	//
	// The server accepts fifty requests in a batch by default, and
	// the limit is configurable. A smaller number is used so that the
	// run works against a server that was never configured.
	BatchSize = 40
	// PageSize is the number of records read in one page.
	//
	// The server accepts a thousand records in a page, and a page of
	// a hundred is enough to keep the memory of the run bounded.
	PageSize = 100
)

// batchRequest is one request of a batch.
//
// It is the shape of the batch API of PocketBase: a method, a path
// and a body. The path is the URL of the records of a collection,
// and the method is the one that upserts.
type batchRequest struct {
	Method string         `json:"method"`
	URL    string         `json:"url"`
	Body   map[string]any `json:"body"`
}

// UpsertRecords writes records on the remote server.
//
// The records are written in batches, and every batch is one request
// to the batch API of the server. The batch API upserts by the
// identifier of the record, so a record that is already there is
// updated and one that is not is created.
//
// The batch API is disabled on a server by default, and the server
// answers a batch that is not allowed with a 403. That error is
// reported as it is: the run needs the batch API, and the user has
// to enable it on the server.
func (c *Client) UpsertRecords(ctx context.Context, collection string, records []RemoteRecord) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	written := 0
	for start := 0; start < len(records); start += BatchSize {
		end := start + BatchSize
		if end > len(records) {
			end = len(records)
		}

		batch := make([]batchRequest, 0, end-start)
		for _, record := range records[start:end] {
			batch = append(batch, batchRequest{
				Method: http.MethodPut,
				URL:    "/api/collections/" + url.PathEscape(collection) + "/records",
				Body:   record,
			})
		}

		payload := map[string]any{"requests": batch}
		raw, status, err := c.do(ctx, http.MethodPost, "/batch", nil, payload)
		if err != nil {
			return written, err
		}
		if status != http.StatusOK {
			if status == http.StatusForbidden {
				return written, fmt.Errorf(
					"sync: the remote server refused a batch of %s: the batch API is not enabled on it",
					collection)
			}
			return written, fmt.Errorf("sync: the remote server answered %d to a batch of %s: %s",
				status, collection, summarise(raw))
		}

		// The batch answers with the result of every request. One of
		// them failing fails the batch, because the server runs it in
		// one transaction, but the answer is read anyway so that the
		// error names the record that failed.
		var results []struct {
			Status int `json:"status"`
			Body   any `json:"body"`
		}
		if err := json.Unmarshal(raw, &results); err == nil {
			for _, result := range results {
				if result.Status >= 400 {
					return written, fmt.Errorf("sync: the remote server refused a record of %s with %d",
						collection, result.Status)
				}
			}
		}

		written += end - start
	}

	return written, nil
}

// ListRecords returns the records of a collection that changed since
// a time.
//
// The filter is the one of the API, and it is applied by the server:
// the run reads a page at a time and never holds the whole collection
// in memory.
//
// The filter is on the field the sync owns, and not on the "updated"
// field of the server: the server overwrites that one on every write,
// so it says when the server was written to and not when the record
// was changed.
//
// The value of the filter is written into the filter itself and is
// not a placeholder: the API of the server binds the placeholders of
// a filter to the parameters of the query, and there is no way to
// bind one from the outside of the request.
func (c *Client) ListRecords(ctx context.Context, collection string, since time.Time, limit int) ([]RemoteRecord, error) {
	if limit <= 0 {
		limit = PageSize
	}

	out := make([]RemoteRecord, 0, limit)
	page := 1

	for len(out) < limit {
		query := url.Values{}
		if !since.IsZero() {
			query.Set("filter", fmt.Sprintf("%s > '%s'", FieldSourceUpdated, formatDateTime(since)))
		}
		query.Set("perPage", strconv.Itoa(min(limit-len(out), PageSize)))
		query.Set("page", strconv.Itoa(page))
		// The count of the records is not asked for: the run only
		// needs to know when a page is short.
		query.Set("skipTotal", "1")

		raw, status, err := c.do(ctx, http.MethodGet,
			"/collections/"+url.PathEscape(collection)+"/records", query, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("sync: the remote server answered %d to the list of the records of %s: %s",
				status, collection, summarise(raw))
		}

		var answer struct {
			Items []RemoteRecord `json:"items"`
		}
		if err := json.Unmarshal(raw, &answer); err != nil {
			return nil, fmt.Errorf("sync: decode the records of %s: %w", collection, err)
		}

		for _, record := range answer.Items {
			// The filter of the server is applied on the field of
			// the sync, and the value of the field is compared
			// again here: the server compares the strings, and the
			// comparison of the strings is not the comparison of the
			// times when the formats differ.
			if !since.IsZero() && !record.SourceUpdated().After(since) {
				continue
			}
			out = append(out, record)
		}

		if len(answer.Items) < PageSize {
			break
		}
		page++
	}

	return out, nil
}

// HasCollection tells whether the remote server has a collection.
//
// The check is made against the list of the collections, because the
// answer of the API of a collection that is not there is a 404 that
// says less than the list does.
func (c *Client) HasCollection(ctx context.Context, name string) (bool, error) {
	collections, err := c.Collections(ctx)
	if err != nil {
		return false, err
	}
	for _, collection := range collections {
		if collection.Name() == name {
			return true, nil
		}
	}
	return false, nil
}
