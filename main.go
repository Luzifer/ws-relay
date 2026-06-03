// ws-relay webservice
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Luzifer/rconfig/v2"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const websocketBufferSize = 1024

var (
	cfg = struct {
		Listen              string `flag:"listen" default:":3000" description:"Port/IP to listen on"`
		LogLevel            string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		MaxConnsPerSocket   int    `flag:"max-conns-per-socket" default:"100" description:"How many connections to allow per socket URI, zero to disable limits"`
		MaxMessageSizeBytes int64  `flag:"max-message-size-bytes" default:"1048576" description:"Maximum message size in Byte, zero to disable limits"`
		VersionAndExit      bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	upgrader = websocket.Upgrader{
		CheckOrigin:     func(*http.Request) bool { return true },
		ReadBufferSize:  websocketBufferSize,
		WriteBufferSize: websocketBufferSize,
	}

	version = "dev"
)

func initApp() error {
	rconfig.AutoEnv(true)
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		return fmt.Errorf("parsing cli options: %w", err)
	}

	l, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("parsing log-level: %w", err)
	}
	logrus.SetLevel(l)

	if cfg.MaxConnsPerSocket < 0 {
		return fmt.Errorf("max-conns-per-socket cannot be negative")
	}

	if cfg.MaxMessageSizeBytes < 0 {
		return fmt.Errorf("max-message-size-bytes cannot be negative")
	}

	return nil
}

func main() {
	var err error
	if err = initApp(); err != nil {
		logrus.WithError(err).Fatal("initializing app")
	}

	if cfg.VersionAndExit {
		fmt.Printf("ws-relay %s\n", version) //nolint:forbidigo // Printing version to stdout is fine
		os.Exit(0)
	}

	logrus.WithField("version", version).Info("ws-relay started")

	router := mux.NewRouter()
	router.HandleFunc("/{socket}", handleSocketRelay)

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           router,
		ReadHeaderTimeout: time.Second,
	}

	if err = server.ListenAndServe(); err != nil {
		logrus.WithError(err).Fatal("running HTTP server")
	}
}

func handleSocketRelay(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		// That's no socket request, don't spam the logs
		http.Error(w, "this is a socket", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.WithError(err).Error("upgrading socket")
		return
	}
	conn.SetReadLimit(cfg.MaxMessageSizeBytes)

	var (
		socketName = mux.Vars(r)["socket"]
		logger     = logrus.WithField("socket", socketName)
	)

	connID, unregister, err := pool.Register(socketName, conn, cfg.MaxConnsPerSocket)
	if err != nil {
		logger.WithError(err).Warn("registering socket")
		if err := conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "connection limit reached"),
			time.Now().Add(websocketWriteTimeout),
		); err != nil {
			logger.WithError(err).Debug("sending socket close message")
		}
		if err := conn.Close(); err != nil {
			logger.WithError(err).Error("closing socket connection (leaked fd)")
		}
		return
	}

	logger = logger.WithField("id", connID)
	defer unregister()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNoStatusReceived, websocket.CloseNormalClosure) {
				logger.WithError(err).Debug("socket disconnected")
				return
			}

			logger.WithError(err).Error("reading from connection")
			return
		}

		pool.Send(socketName, msgType, msg)
	}
}
