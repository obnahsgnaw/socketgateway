package socket

import (
	"sync"
)

type IDManager struct {
	mu   sync.RWMutex
	data map[string][]int
}

func newIDManager() *IDManager {
	return &IDManager{
		data: make(map[string][]int),
	}
}

func (m *IDManager) Add(id string, fd int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list, exists := m.data[id]
	if !exists {
		m.data[id] = []int{fd}
		return
	}

	for _, existingFD := range list {
		if existingFD == fd {
			return
		}
	}

	m.data[id] = append(list, fd)
}

func (m *IDManager) Get(id string) []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list, exists := m.data[id]
	if !exists {
		return nil
	}

	copyList := make([]int, len(list))
	copy(copyList, list)
	return copyList
}

func (m *IDManager) Del(id string, fd int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list, exists := m.data[id]
	if !exists {
		return
	}

	index := -1
	for i, existingFD := range list {
		if existingFD == fd {
			index = i
			break
		}
	}
	if index != -1 {
		m.data[id] = append(list[:index], list[index+1:]...)
		if len(m.data[id]) == 0 {
			delete(m.data, id)
		}
	}
}
