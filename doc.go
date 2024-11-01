package socketgateway

import (
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/application/service/regCenter"
	http2 "github.com/obnahsgnaw/http"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/doc"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	url2 "net/url"
	"strconv"
)

/*
1. 注册当前 socket gateway 的路由 以及 带API网关前缀的路由
2. 注册当前 socket gateway 以及 socket handler的模块路由，显示模块下的文档
3. 注册当前模块下的特定key的路由，显示具体文档
*/

type DocConfig struct {
	id       string
	endType  endtype.EndType
	servType servertype.ServerType
	RegTtl   int64
	CacheTtl int
	Doc      DocItem
}

type DocItem struct {
	Title      string
	Public     bool
	socketType sockettype.SocketType // 通过上层应用来设置
	path       string
	Provider   func() ([]byte, error)
}

type DocServer struct {
	config  *DocConfig
	engine  *http2.Http
	Manager *doc.Manager
	regInfo *regCenter.RegInfo
	prefix  string
	origin  url.Origin
}

// doc-index --> id-list --> key list

func newDocServerWithEngine(e *http2.Http, clusterId string, config *DocConfig) *DocServer {
	s := &DocServer{
		config:  config,
		engine:  e,
		Manager: doc.NewManager(),
		prefix:  utils.ToStr("/", config.endType.String(), "-", config.servType.String(), "-docs"), // the same prefix with the socket handler
		origin: url.Origin{
			Protocol: url.HTTP,
			Host: url.Host{
				Ip:   e.Ip(),
				Port: e.Port(),
			},
		},
	}
	config.Doc.path = s.prefix + "/gateway/gateway"
	public := "0"
	if config.Doc.Public {
		public = "1"
	}
	s.regInfo = &regCenter.RegInfo{
		AppId:   clusterId,
		RegType: regtype.Doc,
		ServerInfo: regCenter.ServerInfo{
			Id:      config.id,
			Name:    config.Doc.Title,
			Type:    config.Doc.socketType.String(),
			EndType: config.endType.String(),
		},
		Host: s.engine.Host(),
		Val:  "",
		Ttl:  config.RegTtl,
		Values: map[string]string{
			"title":  config.Doc.Title,
			"url":    s.DocUrl(),
			"public": public,
		},
	}
	if err := s.initTemplate(); err != nil {
		log.Println(err.Error())
	}
	s.initModuleRoute()
	s.initKeyRoute()

	return s
}

func (s *DocServer) RegInfo() *regCenter.RegInfo {
	return s.regInfo
}

// gateway及handler 文档主页模版
func (s *DocServer) initTemplate() error {
	t := template.New("socket-gw-index.tmpl")
	tmpl, _ := doc.Assets.ReadFile("html/index.tmpl")
	_, err := t.Parse(string(tmpl))
	if err != nil {
		return err
	}
	s.engine.Engine().SetHTMLTemplate(t)
	return nil
}

// gateway及handler 模块主页
func (s *DocServer) initModuleRoute() {
	// 一个模块的文档（便于一个模块一个模块提供而不是全部提供）
	hd := func(c *gin.Context) {
		var gwUrls = make(doc.ModuleDoc)
		// 非gateway模块 一直显示网关文档
		if c.Param("md") != "gateway" {
			gwUrls, _ = s.Manager.GetModuleDocs("gateway")
		}
		admin := c.Query("admin") == "cptbtptp"
		c.Header("Cache-control", "private,max-age=86400")
		urls, title := s.Manager.GetModuleDocs(c.Param("md"))
		publicUrls := make(doc.ModuleDoc)
		if admin {
			publicUrls = urls
		} else {
			for k, item := range urls {
				if item.Public {
					publicUrls[k] = item
				}
			}
		}
		c.HTML(http.StatusOK, "socket-gw-index.tmpl", gin.H{
			"module":  c.Param("md"),
			"title":   title,
			"gateway": gwUrls,
			"urls":    publicUrls,
		})
	}
	s.engine.Engine().GET(s.prefix+"/:md", hd) //gateway及handler 模块主页路由
}

func (s *DocServer) docHd(c *gin.Context) {
	if s.config.Doc.Provider == nil {
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write([]byte("no doc provider"))
		return
	}
	tmpl, err := s.config.Doc.Provider()
	if err != nil {
		c.String(404, err.Error())
	} else {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(tmpl)
	}
}

func (s *DocServer) initKeyRoute() {
	hd := func(c *gin.Context) {
		cacheTtl := s.config.CacheTtl
		if cacheTtl <= 0 {
			cacheTtl = 600
		}
		if c.Param("md") == "gateway" {
			s.docHd(c)
			return
		}
		k := c.Param("key")
		c.Header("Cache-control", "private,max-age="+strconv.Itoa(cacheTtl))
		if addr := s.Manager.GetRandKeyDoc(c.Param("md"), k); addr == "" {
			c.String(404, "Sub doc url not found.")
		} else {
			p := proxy(addr)
			dr := p.Director
			p.Director = func(request *http.Request) {
				dr(request)
				request.URL.Path = c.Request.URL.Path
				request.URL.RawPath = c.Request.URL.RawPath
			}
			p.ServeHTTP(c.Writer, c.Request)
		}
	}
	s.engine.Engine().GET(s.prefix+"/:md/:key", hd) // key 路由
}

func (s *DocServer) Start(key string, cb func(error)) {
	s.engine.RunAndServWithKey(key, cb)
}

func (s *DocServer) DocUrl() string {
	return s.origin.String() + s.config.Doc.path
}

func (s *DocServer) IndexDocUrl() string {
	return s.origin.String() + s.prefix + "/:md"
}

func (s *DocServer) SyncStart(key string, cb func(error)) {
	go func() {
		defer s.engine.CloseWithKey(key)
		s.Start(key, cb)
	}()
}

func proxy(origin string) *httputil.ReverseProxy {
	target, _ := url2.Parse(origin)
	p := httputil.NewSingleHostReverseProxy(target)
	return p
}
