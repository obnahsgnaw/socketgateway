package doc

import (
	"github.com/obnahsgnaw/application/pkg/utils"
	"sync"
)

type Manager struct {
	sync.Mutex
	docs map[module]ModuleDoc
}

type module string
type key string
type ModuleDoc map[key]*item
type item struct {
	Title string
	urls  map[string]struct{}
}

func NewManager() *Manager {
	return &Manager{
		docs: make(map[module]ModuleDoc),
	}
}

func (m *Manager) Add(moduleName, keyName, title, url string) {
	m.Lock()
	defer m.Unlock()

	mk := module(moduleName)
	k := key(keyName)

	if _, ok := m.docs[mk]; !ok {
		m.docs[mk] = make(ModuleDoc)
	}

	if _, ok := m.docs[mk][k]; ok {
		if title != "" {
			m.docs[mk][k].Title = title
		}
		if url != "" {
			m.docs[mk][k].urls[url] = struct{}{}
		}
	} else {
		m.docs[mk][k] = &item{
			Title: title,
			urls:  map[string]struct{}{url: {}},
		}
	}
}

func (m *Manager) Remove(moduleName, keyName, url string) {
	mk := module(moduleName)
	k := key(keyName)
	if _, ok := m.docs[mk]; ok {
		if _, ok = m.docs[mk][k]; ok {
			delete(m.docs[mk][k].urls, url)
			if len(m.docs[mk][k].urls) == 0 {
				delete(m.docs[mk], k)
			}
		}
	}
}

func (m *Manager) GetModuleDocs(moduleName string) ModuleDoc {
	mk := module(moduleName)
	doc := make(ModuleDoc)
	if _, ok := m.docs[mk]; ok {
		doc = m.docs[mk]
	}
	return doc
}

func (m *Manager) GetKeyDocs(moduleName, keyName string) (docs []string) {
	mk := module(moduleName)
	k := key(keyName)
	if _, ok := m.docs[mk]; ok {
		if _, ok = m.docs[mk][k]; ok {
			for u := range m.docs[mk][k].urls {
				docs = append(docs, u)
			}
		}
	}
	return
}

func (m *Manager) GetRandKeyDoc(moduleName, keyName string) string {
	list := m.GetKeyDocs(moduleName, keyName)
	if len(list) == 0 {
		return ""
	}
	return list[utils.RandInt(len(list))]
}
