package main

import (
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const websocketWriteTimeout = 5 * time.Second

type (
	socketPool struct {
		lock sync.RWMutex
		pool map[string]map[string]*socketPoolEntry
	}

	socketPoolEntry struct {
		conn *websocket.Conn
		lock sync.Mutex
	}
)

var pool = newSocketPool()

func newSocketPool() *socketPool {
	return &socketPool{
		pool: make(map[string]map[string]*socketPoolEntry),
	}
}

func (s *socketPool) Register(name string, conn *websocket.Conn) (string, func()) {
	s.lock.Lock()
	defer s.lock.Unlock()

	connID := uuid.New().String()

	if s.pool[name] == nil {
		s.pool[name] = make(map[string]*socketPoolEntry)
	}

	s.pool[name][connID] = &socketPoolEntry{conn: conn}
	logrus.
		WithFields(logrus.Fields{"id": connID, "socket": name}).
		Info("registered socket")

	return connID, func() { s.Unregister(name, connID) }
}

func (s *socketPool) Send(name string, msgType int, msg []byte) {
	s.lock.RLock()
	targets := make(map[string]*socketPoolEntry, len(s.pool[name]))
	maps.Copy(targets, s.pool[name])
	s.lock.RUnlock()

	wg := new(sync.WaitGroup)

	for id, entry := range targets {
		wg.Add(1)
		go s.SendLocked(wg, name, id, entry, msgType, msg)
	}

	wg.Wait()
}

func (s *socketPool) SendLocked(wg *sync.WaitGroup, name, id string, entry *socketPoolEntry, msgType int, msg []byte) {
	defer wg.Done()

	entry.lock.Lock()
	defer entry.lock.Unlock()

	logger := logrus.WithFields(logrus.Fields{"id": id, "socket": name})

	if err := entry.conn.SetWriteDeadline(time.Now().Add(websocketWriteTimeout)); err != nil {
		logger.WithError(err).Error("setting write deadline")
		s.Unregister(name, id)
		return
	}

	if err := entry.conn.WriteMessage(msgType, msg); err != nil {
		logger.WithError(err).Error("delivering to socket")
		s.Unregister(name, id)
	}
}

func (s *socketPool) Unregister(name, connID string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.pool[name] == nil || s.pool[name][connID] == nil {
		return
	}

	logger := logrus.
		WithFields(logrus.Fields{"id": connID, "socket": name})

	if err := s.pool[name][connID].conn.Close(); err != nil {
		logger.WithError(err).Error("closing socket connection (leaked fd)")
	}
	delete(s.pool[name], connID)

	logger.Info("unregistered socket")
}
