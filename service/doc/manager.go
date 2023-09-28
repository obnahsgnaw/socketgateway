package doc

import (
	"github.com/obnahsgnaw/application/pkg/utils"
	"strings"
	"sync"
)

type Manager struct {
	sync.Mutex
	docs map[module]*ModuleItem
}

type module string
type ModuleItem struct {
	Name string
	Doc  ModuleDoc
}
type key string
type ModuleDoc map[key]*item
type item struct {
	Title  string
	Public bool
	urls   map[string]struct{}
}

func NewManager() *Manager {
	return &Manager{
		docs: make(map[module]*ModuleItem),
	}
}

func (m *Manager) Add(moduleName, keyName, title, url string, public *bool) {
	m.Lock()
	defer m.Unlock()

	mdn, title := parseMdNameKey(title, moduleName)
	mk := module(moduleName)
	k := key(keyName)

	if _, ok := m.docs[mk]; !ok {
		m.docs[mk] = &ModuleItem{
			Name: mdn,
			Doc:  make(ModuleDoc),
		}
	}
	m.docs[mk].Name = mdn

	if _, ok := m.docs[mk].Doc[k]; ok {
		if title != "" {
			m.docs[mk].Doc[k].Title = title
		}
		if url != "" {
			m.docs[mk].Doc[k].urls[url] = struct{}{}
		}
		if public != nil {
			m.docs[mk].Doc[k].Public = *public
		}
	} else {
		pub := false
		if public != nil {
			pub = *public
		}
		m.docs[mk].Doc[k] = &item{
			Title:  title,
			Public: pub,
			urls:   map[string]struct{}{},
		}
		if url != "" {
			m.docs[mk].Doc[k].urls[url] = struct{}{}
		}
	}
}

func (m *Manager) Remove(moduleName, keyName, url string) {
	mk := module(moduleName)
	k := key(keyName)
	if _, ok := m.docs[mk]; ok {
		if _, ok = m.docs[mk].Doc[k]; ok {
			delete(m.docs[mk].Doc[k].urls, url)
			if len(m.docs[mk].Doc[k].urls) == 0 {
				delete(m.docs[mk].Doc, k)
			}
		}
	}
}

func (m *Manager) GetModuleDocs(moduleKey string) (ModuleDoc, string) {
	mk := module(moduleKey)
	doc := make(ModuleDoc)
	title := moduleKey
	if _, ok := m.docs[mk]; ok {
		doc = m.docs[mk].Doc
		title = m.docs[mk].Name
	}
	return doc, title
}

func (m *Manager) GetKeyDocs(moduleName, keyName string) (docs []string) {
	mk := module(moduleName)
	k := key(keyName)
	if _, ok := m.docs[mk]; ok {
		if _, ok = m.docs[mk].Doc[k]; ok {
			for u := range m.docs[mk].Doc[k].urls {
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

func (m *Manager) GetModules(public bool) map[string]string {
	rs := make(map[string]string)
	for mk, it := range m.docs {
		if public {
			hasPub := false
			for _, it1 := range it.Doc {
				if it1.Public {
					hasPub = true
					break
				}
			}
			if hasPub {
				rs[string(mk)] = it.Name
			}
		} else {
			rs[string(mk)] = it.Name
		}
	}
	return rs
}

func parseMdNameKey(title, moduleName string) (string, string) {
	kTitle := title
	mdName := moduleName
	if strings.Contains(title, ":") {
		mdNames := strings.Split(moduleName, ":")
		mdName = mdNames[0]
		kTitle = mdNames[1]
	}
	return mdName, kTitle
}
