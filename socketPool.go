package main

import (
	"path"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var pool = newSocketPool()

type (
	socketPool struct {
		lock      sync.RWMutex
		pool      map[string]map[string]*websocket.Conn
		sendQueue *namedLocker
	}
)

func newSocketPool() *socketPool {
	return &socketPool{
		pool:      make(map[string]map[string]*websocket.Conn),
		sendQueue: newNamedLocker(),
	}
}

func (s *socketPool) Register(name string, conn *websocket.Conn) (string, func()) {
	s.lock.Lock()
	defer s.lock.Unlock()

	connID := uuid.Must(uuid.NewV4()).String()

	if s.pool[name] == nil {
		s.pool[name] = map[string]*websocket.Conn{}
	}

	s.pool[name][connID] = conn
	logrus.
		WithFields(logrus.Fields{"id": connID, "socket": name}).
		Info("registered socket")

	return connID, func() { s.Unregister(name, connID) }
}

func (s *socketPool) Send(name string, msgType int, msg []byte) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	wg := new(sync.WaitGroup)

	for id := range s.pool[name] {
		wg.Add(1)
		go s.SendLocked(wg, name, id, msgType, msg)
	}

	wg.Wait()
}

func (s *socketPool) SendLocked(wg *sync.WaitGroup, name, id string, msgType int, msg []byte) {
	defer wg.Done()

	s.sendQueue.Lock(path.Join(name, id))
	defer s.sendQueue.Unlock(path.Join(name, id))

	if err := s.pool[name][id].WriteMessage(msgType, msg); err != nil {
		logrus.
			WithError(err).
			WithFields(logrus.Fields{"id": id, "socket": name}).
			Error("delivering to socket")
		s.Unregister(name, id)
	}
}

func (s *socketPool) Unregister(name, connID string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.pool[name] == nil || s.pool[name][connID] == nil {
		return
	}

	s.pool[name][connID].Close()
	delete(s.pool[name], connID)

	logrus.
		WithFields(logrus.Fields{"id": connID, "socket": name}).
		Info("unregistered socket")
}
