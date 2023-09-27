package socketgateway

import (
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/application/service/regCenter"
	"github.com/obnahsgnaw/socketgateway/asset"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/doc"
	"html/template"
	"net/http"
	"net/http/httputil"
	url2 "net/url"
	"strconv"
	"strings"
)

type DocConfig struct {
	id            string
	endType       endtype.EndType
	servType      servertype.ServerType
	Origin        url.Origin
	RegTtl        int64
	Prefix        string
	socketGateway bool
	CacheTtl      int
	Doc           DocItem
	debug         bool
}

type DocItem struct {
	Title      string
	Public     bool
	socketType sockettype.SocketType // 通过上层应用来设置
	Path       string
	Prefix     string
	Provider   func() ([]byte, error)
}

type DocServer struct {
	config    *DocConfig
	engine    *gin.Engine
	Manager   *doc.Manager
	regInfo   *regCenter.RegInfo
	moduleDoc string
}

// doc-index --> id-list --> key list

// NewDocServer new a socket doc server
func NewDocServer(clusterId string, config *DocConfig) *DocServer {
	if config.debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	e := gin.Default()
	e.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	s := &DocServer{
		config:    config,
		engine:    e,
		Manager:   doc.NewManager(),
		moduleDoc: "/docs/:md", // the same prefix with the socket handler
	}
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
		Host: config.Origin.Host.String(),
		Val:  "",
		Ttl:  config.RegTtl,
		Values: map[string]string{
			"title":  config.Doc.Title,
			"url":    s.DocUrl(),
			"public": public,
		},
	}

	return s
}

func (s *DocServer) RegInfo() *regCenter.RegInfo {
	return s.regInfo
}

func (s *DocServer) initTemplate() error {
	t := template.New("index.tmpl")
	tmpl, _ := asset.Asset("service/doc/html/index.tmpl")
	_, err := t.Parse(string(tmpl))
	if err != nil {
		return err
	}
	s.engine.SetHTMLTemplate(t)
	return nil
}

func (s *DocServer) initIndexRoute() {
	// 一个模块的文档（便于一个模块一个模块提供而不是全部提供）
	hd := func(c *gin.Context) {
		var gwUrls = make(doc.ModuleDoc)
		// 非网管模块 一直显示网关文档
		if c.Param("md") != "gateway" {
			gwUrls = s.Manager.GetModuleDocs("gateway")
		}
		admin := c.Query("admin") == "cptbtptp"
		c.Header("Cache-control", "private,max-age=86400")
		urls := s.Manager.GetModuleDocs(c.Param("md"))
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
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"module":  c.Param("md"),
			"gateway": gwUrls,
			"urls":    publicUrls,
			"suf":     "." + s.config.servType.String() + "doc",
		})
	}
	s.engine.GET(s.moduleDoc, hd)
	if s.config.Prefix != "" {
		s.engine.GET(s.config.Prefix+"/"+s.moduleDoc, hd)
	}
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

func (s *DocServer) initDocRoute() {
	s.engine.GET(s.config.Doc.Path, s.docHd)
	if s.config.Doc.Prefix != "" {
		s.engine.GET(s.config.Doc.Prefix+"/"+s.config.Doc.Path, s.docHd)
	}
}

func (s *DocServer) initServerProxyDocRoute() {
	hd := func(c *gin.Context) {
		cacheTtl := s.config.CacheTtl
		if cacheTtl <= 0 {
			cacheTtl = 600
		}
		if c.Param("md") == "gateway" {
			s.docHd(c)
			return
		}
		k := strings.TrimSuffix(c.Param("key"), "."+s.config.servType.String()+"doc")
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
	s.engine.GET(s.moduleDoc+"/:key", hd)
	if s.config.Prefix != "" {
		s.engine.GET(s.config.Prefix+"/"+s.moduleDoc+"/:key", hd)
	}
}

func (s *DocServer) Start() error {
	s.initDocRoute()
	if s.config.socketGateway {
		if err := s.initTemplate(); err != nil {
			return err
		}
		s.initIndexRoute()
		s.initServerProxyDocRoute()
	}
	if err := s.engine.Run(s.config.Origin.Host.String()); err != nil {
		return err
	}
	return nil
}

func (s *DocServer) DocUrl() string {
	return s.config.Origin.String() + s.config.Doc.Path
}

func (s *DocServer) IndexDocUrl() string {
	return s.config.Origin.String() + s.moduleDoc
}

func (s *DocServer) ModuleDocUrl(module string) string {
	return s.config.Origin.String() + strings.Replace(s.moduleDoc, ":md", module, 1)
}

func (s *DocServer) SyncStart(cb func(error)) {
	go func() {
		if err := s.Start(); err != nil {
			cb(err)
		}
	}()
}

func proxy(origin string) *httputil.ReverseProxy {
	target, _ := url2.Parse(origin)
	p := httputil.NewSingleHostReverseProxy(target)
	return p
}
