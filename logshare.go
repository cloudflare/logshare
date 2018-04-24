package logshare

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	apiURL     = "https://api.cloudflare.com/client/v4"
	byRequest  = "requests"
	byReceived = "received"
)

const (
	unix     = "unix"
	unixNano = "unixnano"
	rfc3339  = "rfc3339"
)

// Client holds the current API credentials & HTTP client configuration. Client
// should not be modified concurrently.
type Client struct {
	endpoint        string
	apiKey          string
	apiEmail        string
	byReceived      bool
	sample          float64
	timestampFormat string
	fields          []string
	httpClient      *http.Client
	dest            io.Writer
	headers         http.Header
}

// Options for configuring log retrieval requests.
type Options struct {
	// Provide a custom HTTP client. Defaults to a barebones *http.Client.
	HTTPClient *http.Client
	// Provide custom HTTP request headers.
	Headers http.Header
	// Destination to stream logs to.
	Dest io.Writer
	// Multiple destinations to stream logs to. Both Dest and MultiDest can be used (Dest + MultiDest).
	MultiDest []io.Writer
	// Fetch logs by the processing/received timestamp
	ByReceived bool
	// Which timestamp format to use: one of "unix", "unixnano", "rfc3339"
	TimestampFormat string
	// Whether to only retrieve a sample of logs (0.1 to 0.9)
	Sample float64
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

	// Default to the received endpoint.
	var byReceived = true
	if options != nil {
		byReceived = options.ByReceived
	}

	client := &Client{
		apiKey:     apiKey,
		apiEmail:   apiEmail,
		endpoint:   apiURL,
		httpClient: http.DefaultClient,
		dest:       io.MultiWriter(os.Stdout),
		headers:    make(http.Header),
		byReceived: byReceived,
	}

	if options != nil {
		client.timestampFormat = options.TimestampFormat
		client.sample = options.Sample

		if options.Dest != nil {
			if options.MultiDest == nil {
				options.MultiDest = make([]io.Writer, 0, 1)
			}
			options.MultiDest = append(options.MultiDest, options.Dest)
		}
		if options.MultiDest != nil {
			client.dest = io.MultiWriter(options.MultiDest...)
		}

		if options.Fields != nil {
			client.fields = options.Fields
		}
	}

	return client, nil
}

func (c *Client) buildURL(zoneID string, params url.Values) (*url.URL, error) {
	endpoint := byReceived
	if !c.byReceived {
		endpoint = byRequest
	}

	u, err := url.Parse(
		fmt.Sprintf("%s/zones/%s/logs/%s",
			c.endpoint,
			zoneID,
			endpoint,
		),
	)
	if err != nil {
		return nil, err
	}

	if c.byReceived && len(c.fields) >= 1 {
		params.Set("fields", strings.Join(c.fields, ","))
	}

	if c.sample != 0.0 {
		params.Set("sample", strconv.FormatFloat(c.sample, 'f', 1, 64))
	}

	if c.timestampFormat != "" {
		params.Set("timestamps", c.timestampFormat)
	}

	u.RawQuery = params.Encode()
	return u, nil
}

// GetFromTimestamp fetches logs between the start and end timestamps provided,
// (up to 'count' logs).
func (c *Client) GetFromTimestamp(zoneID string, start int64, end int64, count int) (*Meta, error) {
	params := url.Values{}
	params.Set("start", strconv.FormatInt(start, 10))

	if end > 0 {
		params.Set("end", strconv.FormatInt(end, 10))
	}

	if count > 0 {
		params.Set("count", strconv.Itoa(count))
	}

	u, err := c.buildURL(zoneID, params)
	if err != nil {
		return nil, err
	}

	return c.request(u)
}

// FetchFieldNames fetches the names of the available log fields.
func (c *Client) FetchFieldNames(zoneID string) (*Meta, error) {
	u, err := url.Parse(
		fmt.Sprintf(
			"%s/zones/%s/logs/received/fields",
			c.endpoint,
			zoneID,
		),
	)
	if err != nil {
		return nil, err
	}
	return c.request(u)
}

func (c *Client) request(u *url.URL) (*Meta, error) {
	req, err := http.NewRequest("GET", u.String(), nil)
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
		URL:        u.String(),
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Read errors, but provide a cap on total read size for safety.
		lr := io.LimitReader(resp.Body, 1000000)
		body, err := ioutil.ReadAll(lr)
		if err != nil {
			return meta, errors.Wrapf(err, "HTTP status %d: request failed", resp.StatusCode)
		}

		return meta, errors.Errorf("HTTP status %d: request failed: %s", resp.StatusCode, body)
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
