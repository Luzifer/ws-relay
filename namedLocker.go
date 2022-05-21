package main

import "sync"

type (
	namedLocker struct {
		lockers map[string]*sync.Mutex
		self    *sync.Mutex
	}
)

func newNamedLocker() *namedLocker {
	return &namedLocker{
		lockers: make(map[string]*sync.Mutex),
		self:    new(sync.Mutex),
	}
}

func (n *namedLocker) Lock(name string) {
	n.getLocker(name).Lock()
}

func (n *namedLocker) Unlock(name string) {
	n.getLocker(name).Unlock()
}

func (n *namedLocker) getLocker(name string) *sync.Mutex {
	n.self.Lock()
	defer n.self.Unlock()

	if n.lockers[name] == nil {
		n.lockers[name] = new(sync.Mutex)
	}

	return n.lockers[name]
}
