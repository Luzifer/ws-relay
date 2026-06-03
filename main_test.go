package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSocketRelayIsolatesSockets(t *testing.T) {
	url := newTestRelay(t, 10, 1024)

	connA := dialTestSocket(t, url, "room-a")
	connB := dialTestSocket(t, url, "room-b")

	require.NoError(t, connA.WriteMessage(websocket.TextMessage, []byte("hello-a")))

	assert.Equal(t, "hello-a", readTestMessage(t, connA))
	assertNoTestMessage(t, connB)
}

func TestHandleSocketRelayRejectsConnectionOverLimit(t *testing.T) {
	url := newTestRelay(t, 1, 1024)

	conn := dialTestSocket(t, url, "limited")
	limitedConn := dialTestSocket(t, url, "limited")

	_, _, err := limitedConn.ReadMessage()
	require.Error(t, err)
	assert.True(t, websocket.IsCloseError(err, websocket.ClosePolicyViolation))

	require.NoError(t, conn.Close())
}

func TestHandleSocketRelayRejectsOversizedMessage(t *testing.T) {
	url := newTestRelay(t, 10, 4)

	conn := dialTestSocket(t, url, "limited-message")

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("oversized")))

	_, _, err := conn.ReadMessage()
	require.Error(t, err)
	assert.True(t, websocket.IsCloseError(err, websocket.CloseMessageTooBig))
}

func TestHandleSocketRelayRelaysWithinSocket(t *testing.T) {
	url := newTestRelay(t, 10, 1024)

	connA := dialTestSocket(t, url, "room")
	connB := dialTestSocket(t, url, "room")

	require.NoError(t, connA.WriteMessage(websocket.TextMessage, []byte("hello")))

	assert.Equal(t, "hello", readTestMessage(t, connA))
	assert.Equal(t, "hello", readTestMessage(t, connB))
}

func TestSocketPoolRegisterLimit(t *testing.T) {
	pool := newSocketPool()

	connA := newTestServerConn(t)
	connB := newTestServerConn(t)
	connC := newTestServerConn(t)
	connD := newTestServerConn(t)
	connE := newTestServerConn(t)

	_, unregisterA, err := pool.Register("limited", connA, 1)
	require.NoError(t, err)
	t.Cleanup(unregisterA)

	_, unregisterB, err := pool.Register("limited", connB, 1)
	require.Error(t, err)
	assert.Nil(t, unregisterB)

	_, unregisterC, err := pool.Register("other", connC, 1)
	require.NoError(t, err)
	t.Cleanup(unregisterC)

	_, unregisterD, err := pool.Register("unlimited", connD, 0)
	require.NoError(t, err)
	t.Cleanup(unregisterD)

	_, unregisterE, err := pool.Register("unlimited", connE, 0)
	require.NoError(t, err)
	t.Cleanup(unregisterE)
}

func TestSocketPoolUnregisterDeletesEmptySocket(t *testing.T) {
	pool := newSocketPool()
	conn := newTestServerConn(t)

	_, unregister, err := pool.Register("room", conn, 1)
	require.NoError(t, err)
	require.Contains(t, pool.pool, "room")

	unregister()

	assert.NotContains(t, pool.pool, "room")
}

func assertNoTestMessage(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	_, _, err := conn.ReadMessage()
	require.Error(t, err)
}

func dialTestSocket(t *testing.T, baseURL, socketName string) *websocket.Conn {
	t.Helper()

	conn, resp, err := websocket.DefaultDialer.Dial(baseURL+"/"+socketName, nil)
	if resp != nil && resp.Body != nil {
		require.NoError(t, resp.Body.Close())
	}
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

func newTestRelay(t *testing.T, maxConnsPerSocket int, maxMessageSizeBytes int64) string {
	t.Helper()

	oldCfg := cfg
	oldPool := pool

	cfg.MaxConnsPerSocket = maxConnsPerSocket
	cfg.MaxMessageSizeBytes = maxMessageSizeBytes
	pool = newSocketPool()

	router := mux.NewRouter()
	router.HandleFunc("/{socket}", handleSocketRelay)

	server := httptest.NewServer(router)
	t.Cleanup(func() {
		server.Close()
		pool = oldPool
		cfg = oldCfg
	})

	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func newTestServerConn(t *testing.T) *websocket.Conn {
	t.Helper()

	var cleanupOnce sync.Once

	connC := make(chan *websocket.Conn, 1)
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		connC <- conn
		<-done
	}))

	clientConn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if resp != nil && resp.Body != nil {
		require.NoError(t, resp.Body.Close())
	}
	require.NoError(t, err)

	serverConn := <-connC
	t.Cleanup(func() {
		cleanupOnce.Do(func() {
			close(done)
			_ = clientConn.Close()
			_ = serverConn.Close()
			server.Close()
		})
	})

	return serverConn
}

func readTestMessage(t *testing.T, conn *websocket.Conn) string {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	msgType, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, msgType)

	return string(msg)
}
