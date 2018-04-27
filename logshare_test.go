package logshare

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const (
	zoneId                = "zone-123-abc"
	startTimestamp        = 154321
	endTimestamp          = 154321
	count                 = 5
	sample                = 0.5
	timeFormat            = "unix"
	apiKey                = "test-api-key"
	accountEmail          = "test@email.io"
	exampleResponse       = `{"ClientIP": "89.163.242.206","ClientRequestHost": "www.theburritobot.com","ClientRequestMethod": "GET","EdgeEndTimestamp": 1506702504461999900,"EdgeResponseBytes": 69045,"EdgeResponseStatus": 200,"EdgeStartTimestamp": 1506702504433000200,"RayID": "3a6050bcbe121a87"}`
	exampleFieldsResponse = `{"ClientASN": "Client AS number", "ClientCountry": "Country of the client IP address", "ClientDeviceType": "Client device type"}`
)

func checkRequestHeaders(t *testing.T, r *http.Request) {
	assert.Equal(t, apiKey, r.Header.Get("X-Auth-Key"))
	assert.Equal(t, accountEmail, r.Header.Get("X-Auth-Email"))
	assert.Equal(t, "application/json", r.Header.Get("Accept"))
}

func getTestClient(ts *httptest.Server) *http.Client {
	cert, err := x509.ParseCertificate(ts.TLS.Certificates[0].Certificate[0])
	if err != nil {
		return nil
	}
	certpool := x509.NewCertPool()
	certpool.AddCert(cert)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certpool,
			},
		},
	}
	return client
}

func TestInvalidClient(t *testing.T) {
	_, err := New("", "", nil)
	assert.EqualError(t, err, "apiKey cannot be empty")
	_, err = New(apiKey, "", nil)
	assert.EqualError(t, err, "apiEmail cannot be empty")
}

func TestClientByRequests(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkRequestHeaders(t, r)
		query := r.URL.Query()
		expectedPath := fmt.Sprintf("/client/v4/zones/%s/logs/requests", zoneId)
		assert.Equal(t, expectedPath, r.URL.EscapedPath())
		assert.Equal(t, fmt.Sprintf("%d", startTimestamp), query.Get("start"))
		assert.Equal(t, fmt.Sprintf("%d", endTimestamp), query.Get("end"))
		assert.Equal(t, fmt.Sprintf("%d", count), query.Get("count"))
		assert.Equal(t, fmt.Sprintf("%.1f", sample), query.Get("sample"))
		assert.Equal(t, timeFormat, query.Get("timestamps"))
		//Generate response with test data
		fmt.Fprintln(w, exampleResponse)
		fmt.Fprintln(w, exampleResponse)
	}))
	defer ts.Close()
	httpClient := getTestClient(ts)

	client, err := New(
		apiKey,
		accountEmail,
		&Options{
			ApiURL:          ts.URL,
			HTTPClient:      httpClient,
			Sample:          sample,
			TimestampFormat: timeFormat,
			Dest:            os.Stdout,
		},
	)
	if err != nil {
		t.Fatal("Failed to create new logshare.Client:", err)
	}
	meta, err := client.GetFromTimestamp(zoneId, startTimestamp, endTimestamp, count)
	if err != nil {
		t.Fatal("Failed to call client.GetFromTimestamp:", err)
	}
	assert.Equal(t, 2, meta.Count)
}

func TestClientFailNoContent(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkRequestHeaders(t, r)
		w.WriteHeader(204)
	}))
	defer ts.Close()
	httpClient := getTestClient(ts)

	client, err := New(
		apiKey,
		accountEmail,
		&Options{
			ApiURL:     ts.URL,
			HTTPClient: httpClient,
		},
	)
	if err != nil {
		t.Fatal("Failed to create new logshare.Client:", err)
	}
	meta, err := client.GetFromTimestamp(zoneId, startTimestamp, endTimestamp, count)
	assert.EqualError(t, err, "HTTP status 204: no logs available. Check that Log Share is enabled for your domain or that you are not attempting to retrieve logs too quickly")
	assert.Equal(t, 0, meta.Count)
	assert.Equal(t, 204, meta.StatusCode)
}

func TestClientByReceivedFields(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkRequestHeaders(t, r)
		query := r.URL.Query()
		expectedPath := fmt.Sprintf("/client/v4/zones/%s/logs/received", zoneId)
		assert.Equal(t, expectedPath, r.URL.EscapedPath())
		assert.Equal(t, "EdgeResponseStatus,ClientIP,RayID", query.Get("fields"))
		//Generate response with test data
		fmt.Fprintln(w, exampleResponse)
	}))
	defer ts.Close()
	httpClient := getTestClient(ts)

	client, err := New(
		apiKey,
		accountEmail,
		&Options{
			ApiURL:     ts.URL,
			HTTPClient: httpClient,
			ByReceived: true,
			Fields:     []string{"EdgeResponseStatus", "ClientIP", "RayID"},
		},
	)
	if err != nil {
		t.Fatal("Failed to create new logshare.Client:", err)
	}
	meta, err := client.GetFromTimestamp(zoneId, startTimestamp, endTimestamp, count)
	if err != nil {
		t.Fatal("Failed to call client.GetFromTimestamp:", err)
	}
	assert.Equal(t, 1, meta.Count)
}

func TestClientFailedOtherError(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkRequestHeaders(t, r)
		w.WriteHeader(500)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer ts.Close()
	httpClient := getTestClient(ts)

	client, err := New(
		apiKey,
		accountEmail,
		&Options{
			ApiURL:     ts.URL,
			HTTPClient: httpClient,
		},
	)
	if err != nil {
		t.Fatal("Failed to create new logshare.Client:", err)
	}
	meta, err := client.FetchFieldNames(zoneId)
	assert.EqualError(t, err, "HTTP status 500: request failed: Internal Server Error")
	assert.Equal(t, 0, meta.Count)
	assert.Equal(t, 500, meta.StatusCode)
}

func TestClientFetchFieldNames(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkRequestHeaders(t, r)
		expectedPath := fmt.Sprintf("/client/v4/zones/%s/logs/received/fields", zoneId)
		assert.Equal(t, expectedPath, r.URL.EscapedPath())
		//Generate response with test data
		fmt.Fprintln(w, exampleFieldsResponse)
	}))
	defer ts.Close()
	httpClient := getTestClient(ts)

	client, err := New(
		apiKey,
		accountEmail,
		&Options{
			ApiURL:     ts.URL,
			HTTPClient: httpClient,
		},
	)
	if err != nil {
		t.Fatal("Failed to create new logshare.Client:", err)
	}
	meta, err := client.FetchFieldNames(zoneId)
	if err != nil {
		t.Fatal("Failed to call client.FetchFieldNames:", err)
	}
	assert.Equal(t, 1, meta.Count)
}
