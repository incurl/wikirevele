package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client talks to the remote PocketBase server.
//
// The server is a PocketBase, and the two operations a sync needs are
// already there: the collections API applies a schema, and the batch
// API upserts the records. The client is the transport: it
// authenticates as a superuser, holds the token, and speaks JSON.
//
// The token is held in memory and is fetched when it is missing. It
// is never written to disk and never returned to the frontend.
type Client struct {
	// baseURL is the root of the remote API, without a trailing
	// slash.
	baseURL string
	// email is the identity of the superuser.
	email string
	// password is the password of the superuser.
	password string
	// http is the transport. It is a field so that a test can
	// replace it.
	http *http.Client

	// mu guards the token.
	mu sync.Mutex
	// bearer is the bearer token of the superuser.
	bearer string
}

// NewClient creates a client for a remote server.
//
// The base URL is the root of the API, for example
// "https://ruis.example.com". The path of the API is added by the
// client.
func NewClient(baseURL, email, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		email:    strings.TrimSpace(email),
		password: password,
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

// SetHTTPClient replaces the transport.
//
// A test uses it to answer without a server. The timeout of the
// default client is short enough for a sync of a small collection and
// long enough for a batch of a large one.
func (c *Client) SetHTTPClient(h *http.Client) {
	if h == nil {
		return
	}
	c.http = h
}

// BaseURL returns the root of the remote API.
func (c *Client) BaseURL() string { return c.baseURL }

// Configured tells whether the client has everything it needs to
// reach the server.
//
// The interface uses it to tell the user what is missing before a
// sync is started.
func (c *Client) Configured() error {
	if c.baseURL == "" {
		return fmt.Errorf("sync: the address of the remote server is not set")
	}
	if c.email == "" {
		return fmt.Errorf("sync: the identity of the superuser is not set")
	}
	if c.password == "" {
		return fmt.Errorf("sync: the password of the superuser is not set")
	}
	return nil
}

// apiURL builds the address of a path of the API.
func (c *Client) apiURL(path string, query url.Values) string {
	full := c.baseURL + "/api" + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	return full
}

// authenticate fetches a token for the superuser.
//
// The token is held until the server refuses it, and a refusal is
// answered with a new token: a token is valid for a day, and a sync
// may well be the first request of the day.
func (c *Client) authenticate(ctx context.Context) (string, error) {
	payload := map[string]string{
		"identity": c.email,
		"password": c.password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("sync: marshal the credentials: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiURL("/collections/_superusers/auth-with-password", nil), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("sync: build the authentication request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("sync: reach %s: %w", c.baseURL, err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("sync: read the authentication response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sync: the remote server refused the credentials: %s", summarise(raw))
	}

	var answer struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return "", fmt.Errorf("sync: decode the authentication response: %w", err)
	}
	if answer.Token == "" {
		return "", fmt.Errorf("sync: the remote server returned no token")
	}

	return answer.Token, nil
}

// token returns a valid token, fetching one when there is none.
//
// The token is fetched once and then reused. A request that is
// refused with a 401 is retried once with a new token, because the
// token may simply have expired.
func (c *Client) bearerToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	current := c.bearer
	c.mu.Unlock()

	if current != "" {
		return current, nil
	}

	fresh, err := c.authenticate(ctx)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.bearer = fresh
	c.mu.Unlock()

	return fresh, nil
}

// invalidate forgets the token, so that the next request fetches a
// new one.
func (c *Client) invalidate() {
	c.mu.Lock()
	c.bearer = ""
	c.mu.Unlock()
}

// do sends a request with the authorization of the superuser.
//
// A 401 is retried once: the token may have expired while the
// application was closed.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, payload any) ([]byte, int, error) {
	// The body is marshaled once and read again on a retry: a request
	// that is sent twice needs the same bytes both times.
	var raw []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("sync: marshal the request: %w", err)
		}
		raw = encoded
	}

	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.bearerToken(ctx)
		if err != nil {
			return nil, 0, err
		}

		var body io.Reader
		if raw != nil {
			body = bytes.NewReader(raw)
		}

		request, err := http.NewRequestWithContext(ctx, method, c.apiURL(path, query), body)
		if err != nil {
			return nil, 0, fmt.Errorf("sync: build the request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", token)
		if raw != nil {
			request.ContentLength = int64(len(raw))
		}

		response, err := c.http.Do(request)
		if err != nil {
			return nil, 0, fmt.Errorf("sync: reach %s: %w", c.baseURL, err)
		}

		raw, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return nil, response.StatusCode, fmt.Errorf("sync: read the response: %w", readErr)
		}

		if response.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.invalidate()
			continue
		}

		return raw, response.StatusCode, nil
	}

	return nil, 0, fmt.Errorf("sync: the request was refused")
}

// summarise shortens a body for an error message.
//
// A refused request can carry a whole page of HTML, and an error
// message that carries it is unreadable in a log.
func summarise(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}

// ping checks that the server is there and that the credentials work.
//
// It is the pre-flight check of a sync: a run that cannot reach the
// server should say so before it touches anything.
func (c *Client) ping(ctx context.Context) error {
	if err := c.Configured(); err != nil {
		return err
	}

	_, status, err := c.do(ctx, http.MethodGet, "/collections", nil, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("sync: the remote server answered %d to the pre-flight check", status)
	}
	return nil
}

// RemoteCollection is a collection of the remote server, as the
// remote server describes it.
//
// It is the shape of a JSON object of the collections API, and it is
// kept loose on purpose: the whole object is carried to the other
// side untouched, so that a field this program does not know about
// still travels.
type RemoteCollection map[string]any

// Name returns the name of the collection.
func (c RemoteCollection) Name() string {
	name, _ := c["name"].(string)
	return name
}

// ID returns the identifier of the collection.
func (c RemoteCollection) ID() string {
	id, _ := c["id"].(string)
	return id
}

// Type returns the type of the collection: "base", "auth" or "view".
func (c RemoteCollection) Type() string {
	kind, _ := c["type"].(string)
	return kind
}

// Collections returns the collections of the remote server.
//
// The system collections are left out: they belong to the server and
// not to the data of the user.
func (c *Client) Collections(ctx context.Context) ([]RemoteCollection, error) {
	raw, status, err := c.do(ctx, http.MethodGet, "/collections", nil, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("sync: the remote server answered %d to the list of the collections", status)
	}

	var answer struct {
		Items []RemoteCollection `json:"items"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return nil, fmt.Errorf("sync: decode the list of the collections: %w", err)
	}

	out := make([]RemoteCollection, 0, len(answer.Items))
	for _, item := range answer.Items {
		if system, _ := item["system"].(bool); system {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// ImportCollections applies a schema on the remote server.
//
// The schema is the JSON of the collections API, and the server
// applies it in one transaction. The collections that are not in the
// list are left alone: deleteMissing is false, because a sync of two
// collections must not delete the third one.
func (c *Client) ImportCollections(ctx context.Context, collections []RemoteCollection) error {
	if len(collections) == 0 {
		return nil
	}

	payload := map[string]any{
		"collections":   collections,
		"deleteMissing": false,
	}

	_, status, err := c.do(ctx, http.MethodPut, "/collections/import", nil, payload)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusOK {
		return fmt.Errorf("sync: the remote server answered %d to the schema import", status)
	}
	return nil
}

// RemoteRecord is a record of the remote server.
//
// The fields are kept as they arrived, and the two fields the sync
// owns are read out of them.
type RemoteRecord map[string]any

// ID returns the identifier of the record.
func (r RemoteRecord) ID() string {
	id, _ := r["id"].(string)
	return id
}

// SourceUpdated returns the time the record was last changed on the
// side it came from.
//
// The value is the string of the API, and an empty value means that
// the record has never been synced.
func (r RemoteRecord) SourceUpdated() time.Time {
	return parseDateTime(r[FieldSourceUpdated])
}

// SourceOrigin returns the side the record was last written on.
func (r RemoteRecord) SourceOrigin() string {
	origin, _ := r[FieldSourceOrigin].(string)
	return origin
}

// parseDateTime reads a date of the API.
//
// The API writes the dates in one format, but a record that was
// written by hand may hold another one, and a value that cannot be
// read is the zero time rather than an error: a record with an
// unreadable date is treated as one that has never been synced.
func parseDateTime(value any) time.Time {
	text, ok := value.(string)
	if !ok || text == "" {
		return time.Time{}
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05.000Z",
		"2006-01-02 15:04:05Z",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

// formatDateTime writes a date in the format of the API.
//
// The format is the one of the default layout of the server, and it
// sorts as a string the same way the times sort: the year, the
// month, the day, the hour, the minute and the second are all in
// their natural order in it.
func formatDateTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05.000Z")
}
