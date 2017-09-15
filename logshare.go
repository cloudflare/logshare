package logshare

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	apiURL     = "https://api.cloudflare.com/client/v4"
	byRequest  = "requests"
	byReceived = "received"
)

// Client holds the current API credentials & HTTP client configuration. Client
// should not be modified concurrently.
type Client struct {
	endpoint   string
	apiKey     string
	apiEmail   string
	byReceived bool
	fields     []string
	httpClient *http.Client
	dest       io.Writer
	headers    http.Header
}

// Options for configuring log retrieval requests.
type Options struct {
	// Provide a custom HTTP client. Defaults to a barebones *http.Client.
	HTTPClient *http.Client
	// Provide custom HTTP request headers.
	Headers http.Header
	// Destination to stream logs to.
	Dest io.Writer
	// Fetch logs by the processing/recieved timestamp
	ByReceived bool
	// The fields to return in the log responses
	Fields []string
}

// Meta contains data about the API response: the number of logs returned,
// the duration of the request, the HTTP status code and the constructed URL.
type Meta struct {
	Count      int
	Duration   int64
	StatusCode int
	URL        string
}

// New creates a new client instance for consuming logs from
// Cloudflare's Enterprise Log Share API. A client should not be modified during
// HTTP requests.
func New(apiKey string, apiEmail string, options *Options) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey cannot be empty")
	}

	if apiEmail == "" {
		return nil, errors.New("apiEmail cannot be empty")
	}

	var byReceived bool
	if options != nil {
		byReceived = options.ByReceived
	}

	client := &Client{
		apiKey:     apiKey,
		apiEmail:   apiEmail,
		endpoint:   apiURL,
		httpClient: http.DefaultClient,
		dest:       os.Stdout,
		headers:    make(http.Header),
		byReceived: byReceived,
	}

	if options != nil && options.Fields != nil {
		client.fields = options.Fields
	}

	if options != nil && options.Dest != nil {
		client.dest = options.Dest
	}

	return client, nil
}

func (c *Client) buildURL(zoneID string) string {
	endpoint := byRequest
	if c.byReceived {
		endpoint = byReceived
	}

	return fmt.Sprintf("%s/zones/%s/logs/%s", c.endpoint, zoneID, endpoint)
}

func (c *Client) addFieldParams(url string) string {
	// The fields param is only supported on the Logpull endpoint
	if !c.byReceived || len(c.fields) < 1 {
		return url
	}

	return url + "&fields=" + strings.Join(c.fields, ",")
}

// GetFromRayID fetches logs for the given rayID, or starting at the given rayID
// if a non-zero end timestamp is provided.
func (c *Client) GetFromRayID(zoneID string, rayID string, end int64, count int) (*Meta, error) {
	url := fmt.Sprintf("%s?start_id=%s", c.buildURL(zoneID), rayID)

	if end > 0 {
		url += fmt.Sprintf("&end=%d", end)
	}

	if count > 0 {
		url += fmt.Sprintf("&count=%d", count)
	}

	url = c.addFieldParams(url)

	return c.request(url)
}

// GetFromTimestamp fetches logs between the start and end timestamps provided,
// (up to 'count' logs).
func (c *Client) GetFromTimestamp(zoneID string, start int64, end int64, count int) (*Meta, error) {
	url := fmt.Sprintf("%s?start=%d", c.buildURL(zoneID), start)

	if end > 0 {
		url += fmt.Sprintf("&end=%d", end)
	}

	if count > 0 {
		url += fmt.Sprintf("&count=%d", count)
	}

	url = c.addFieldParams(url)

	return c.request(url)
}

// FetchFieldNames fetches the names of the available log fields.
func (c *Client) FetchFieldNames(zoneID string) (*Meta, error) {
	url := fmt.Sprintf("%s/zones/%s/logs/received/fields", c.endpoint, zoneID)
	return c.request(url)
}

func (c *Client) request(url string) (*Meta, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a request object")
	}

	// Apply any user-defined headers in a thread-safe manner.
	req.Header = cloneHeader(c.headers)
	req.Header.Set("X-Auth-Key", c.apiKey)
	req.Header.Set("X-Auth-Email", c.apiEmail)
	req.Header.Set("Accept", "application/json")

	start := makeTimestamp()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	meta := &Meta{
		StatusCode: resp.StatusCode,
		Duration:   makeTimestamp() - start,
		URL:        url,
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return meta, errors.Errorf("HTTP status %d: request failed", resp.StatusCode)
	}

	// Explicitly handle the 204 No Content case.
	if resp.StatusCode == 204 {
		return meta, errors.Errorf("HTTP status %d: no logs available. Check that Log Share is enabled for your domain or that you are not attempting to retrieve logs too quickly", resp.StatusCode)
	}

	// Stream the logs from the response to the destination writer.
	meta.Count, err = streamLogs(resp.Body, c.dest)
	if err != nil {
		return meta, errors.Wrap(err, "failed to stream logs")
	}

	return meta, nil
}

// streamLogs streams newline delimited logs to the provided writer, counting
// each newline-delimited JSON log without allocating.
//
// An io.MultiWriter can be created to stream logs to two (or more) different
// sinks: e.g. stdout and a file simultaneously, or a file and a
// http.ResponseWriter.
func streamLogs(r io.Reader, w io.Writer) (int, error) {
	const MB = 1024 * 1024 * 1024
	var count = 0

	scanner := bufio.NewScanner(r)

	// TODO: Consider a buffer pool to read the track the last log read, for
	// checkpointing the rayID.
	for scanner.Scan() {
		w.Write(scanner.Bytes())
		w.Write([]byte("\n"))
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, errors.Wrap(err, "reading response:")
	}

	return count, nil
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

// cloneHeader returns a shallow copy of the header.
// copied from https://godoc.org/github.com/golang/gddo/httputil/header#Copy
func cloneHeader(header http.Header) http.Header {
	h := make(http.Header)
	for k, vs := range header {
		h[k] = vs
	}
	return h
}
