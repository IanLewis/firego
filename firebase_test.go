package firego

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const URL = "https://somefirebaseapp.firebaseIO.com"

type TestServer struct {
	*httptest.Server
	receivedReqs []*http.Request
}

func newTestServer(response string) *TestServer {
	ts := &TestServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ts.receivedReqs = append(ts.receivedReqs, req)
		fmt.Fprint(w, response)
	}))
	return ts
}

func TestNew(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		givenURL string
	}{
		{
			URL,
		},
		{
			URL + "/",
		},
		{
			"somefirebaseapp.firebaseIO.com",
		},
		{
			"somefirebaseapp.firebaseIO.com/",
		},
	}

	for _, tt := range testCases {
		fb := New(tt.givenURL, nil)
		assert.Equal(t, URL, fb.url, "givenURL: %s", tt.givenURL)
	}
}

func TestNewWithProvidedHttpClient(t *testing.T) {
	t.Parallel()

	var tr *http.Transport
	tr = &http.Transport{
		DisableKeepAlives: true, // https://code.google.com/p/go/issues/detail?id=3514
		Dial: func(network, address string) (net.Conn, error) {
			start := time.Now()
			c, err := net.DialTimeout(network, address, TimeoutDuration)
			tr.ResponseHeaderTimeout = TimeoutDuration - time.Since(start)
			return c, err
		},
	}
	var client = http.Client{Transport: tr};

	testCases := []struct {
		givenURL string
	}{
		{
			URL,
		},
		{
			URL + "/",
		},
		{
			"somefirebaseapp.firebaseIO.com",
		},
		{
			"somefirebaseapp.firebaseIO.com/",
		},
	}

	for _, tt := range testCases {
		fb := New(tt.givenURL, &client)
		assert.Equal(t, URL, fb.url, "givenURL: %s", tt.givenURL)
	}
}

func TestChild(t *testing.T) {
	t.Parallel()
	var (
		parent    = New(URL, nil)
		childNode = "node"
		child     = parent.Child(childNode)
	)

	assert.Equal(t, fmt.Sprintf("%s/%s", parent.url, childNode), child.url)
}

func TestChild_Issue26(t *testing.T) {
	t.Parallel()
	parent := New(URL, nil)
	child1 := parent.Child("one")
	child2 := child1.Child("two")

	child1.Shallow(true)
	assert.Len(t, child2.params, 0)
}

func TestTimeoutDuration_Headers(t *testing.T) {
	defer func(dur time.Duration) { TimeoutDuration = dur }(TimeoutDuration)
	TimeoutDuration = time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(2 * TimeoutDuration)
	}))
	defer server.Close()

	fb := New(server.URL, nil)
	err := fb.Value("")
	assert.NotNil(t, err)
	assert.IsType(t, ErrTimeout{}, err)

	// ResponseHeaderTimeout should be TimeoutDuration less the time it took to dial, and should be positive
	require.IsType(t, (*http.Transport)(nil), fb.client.Transport)
	tr := fb.client.Transport.(*http.Transport)
	assert.True(t, tr.ResponseHeaderTimeout < TimeoutDuration)
	assert.True(t, tr.ResponseHeaderTimeout > 0)
}

func TestTimeoutDuration_Dial(t *testing.T) {
	defer func(dur time.Duration) { TimeoutDuration = dur }(TimeoutDuration)
	TimeoutDuration = time.Microsecond

	fb := New("http://dialtimeouterr.or/", nil)
	err := fb.Value("")
	assert.NotNil(t, err)
	assert.IsType(t, ErrTimeout{}, err)

	// ResponseHeaderTimeout should be negative since the total duration was consumed when dialing
	require.IsType(t, (*http.Transport)(nil), fb.client.Transport)
	assert.True(t, fb.client.Transport.(*http.Transport).ResponseHeaderTimeout < 0)
}

func TestShallow(t *testing.T) {
	t.Parallel()
	var (
		server = newTestServer("")
		fb     = New(server.URL, nil)
	)
	defer server.Close()

	fb.Shallow(true)
	fb.Value("")
	require.Len(t, server.receivedReqs, 1)

	req := server.receivedReqs[0]
	assert.Equal(t, shallowParam+"=true", req.URL.Query().Encode())

	fb.Shallow(false)
	fb.Value("")
	require.Len(t, server.receivedReqs, 2)

	req = server.receivedReqs[1]
	assert.Equal(t, "", req.URL.Query().Encode())
}

func TestOrderBy(t *testing.T) {
	t.Parallel()
	var (
		server = newTestServer("")
		fb     = New(server.URL, nil)
	)
	defer server.Close()

	fb.OrderBy("\"user_id\"").Value("")
	require.Len(t, server.receivedReqs, 1)

	req := server.receivedReqs[0]
	assert.Equal(t, orderByParam+"=%22user_id%22", req.URL.Query().Encode())
}

func TestStartAt(t *testing.T) {
	t.Parallel()
	var (
		server = newTestServer("")
		fb     = New(server.URL, nil)
	)
	defer server.Close()

	fb.StartAt("3").Value("")
	require.Len(t, server.receivedReqs, 1)

	req := server.receivedReqs[0]
	assert.Equal(t, startAtParam+"=3", req.URL.Query().Encode())
}

func TestEndAt(t *testing.T) {
	t.Parallel()
	var (
		server = newTestServer("")
		fb     = New(server.URL, nil)
	)
	defer server.Close()

	fb.EndAt("theend").Value("")
	require.Len(t, server.receivedReqs, 1)

	req := server.receivedReqs[0]
	assert.Equal(t, endAtParam+"=theend", req.URL.Query().Encode())
}

func TestIncludePriority(t *testing.T) {
	t.Parallel()
	var (
		server = newTestServer("")
		fb     = New(server.URL, nil)
	)
	defer server.Close()

	fb.IncludePriority(true)
	fb.Value("")
	require.Len(t, server.receivedReqs, 1)

	req := server.receivedReqs[0]
	assert.Equal(t, formatParam+"="+formatVal, req.URL.Query().Encode())

	fb.IncludePriority(false)
	fb.Value("")
	require.Len(t, server.receivedReqs, 2)

	req = server.receivedReqs[1]
	assert.Equal(t, "", req.URL.Query().Encode())
}
