package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/Luzifer/rconfig/v2"
)

const websocketBufferSize = 1024

var (
	cfg = struct {
		Listen         string `flag:"listen" default:":3000" description:"Port/IP to listen on"`
		LogLevel       string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	upgrader = websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  websocketBufferSize,
		WriteBufferSize: websocketBufferSize,
	}

	version = "dev"
)

func initApp() error {
	rconfig.AutoEnv(true)
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		return errors.Wrap(err, "parsing cli options")
	}

	l, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return errors.Wrap(err, "parsing log-level")
	}
	logrus.SetLevel(l)

	return nil
}

func main() {
	var err error
	if err = initApp(); err != nil {
		logrus.WithError(err).Fatal("initializing app")
	}

	if cfg.VersionAndExit {
		fmt.Printf("ws-relay %s\n", version) //nolint:forbidigo
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
	defer func() {
		if err := conn.Close(); err != nil {
			logrus.WithError(err).Error("closing socket connection (leaked fd)")
		}
	}()

	var (
		socketName         = mux.Vars(r)["socket"]
		connID, unregister = pool.Register(socketName, conn)
		logger             = logrus.WithFields(logrus.Fields{"id": connID, "socket": socketName})
	)
	defer unregister()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			logger.WithError(err).Error("reading from connection")
			return
		}

		pool.Send(socketName, msgType, msg)
	}
}
