package builder

import "sync"

type MapMutex struct {
	innerMap map[string]*sync.RWMutex
	mutex    *sync.RWMutex
}

func NewMapMutex() *MapMutex {
	return &MapMutex{
		innerMap: make(map[string]*sync.RWMutex),
		mutex:    &sync.RWMutex{},
	}
}

func (m *MapMutex) getKeyLock(key string) *sync.RWMutex {
	// set read lock to check if key exists
	m.mutex.RLock()

	if lock, ok := m.innerMap[key]; ok {
		m.mutex.RUnlock()
		return lock
	}

	m.mutex.RUnlock()

	// set write lock to add lock to initialize key
	m.mutex.Lock()

	// double check if it hasn't been created in the meantime
	if lock, ok := m.innerMap[key]; ok {
		m.mutex.Unlock()
		return lock
	}

	// otherwise create it
	lock := &sync.RWMutex{}

	m.innerMap[key] = lock
	m.mutex.Unlock()

	// return lock
	return lock
}

func (m *MapMutex) RLock(key string) {
	m.getKeyLock(key).RLock()
}

func (m *MapMutex) RUnlock(key string) {
	m.getKeyLock(key).RUnlock()
}

func (m *MapMutex) Lock(key string) {
	m.getKeyLock(key).Lock()
}

func (m *MapMutex) Unlock(key string) {
	m.getKeyLock(key).Unlock()
}
