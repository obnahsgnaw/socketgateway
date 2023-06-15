package socketgateway

import (
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/service/regCenter"
	"github.com/obnahsgnaw/socketgateway/asset"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/doc"
	"html/template"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
)

type DocConfig struct {
	id            string
	endType       endtype.EndType
	Origin        url.Origin
	RegTtl        int64
	gwPrefix      string // api网关前缀
	socketGateway bool
	CacheTtl      int
	Doc           DocItem
}

type DocItem struct {
	Title      string
	socketType sockettype.SocketType // 通过上层应用来设置
	Path       string
	Provider   func() ([]byte, error)
}

type DocServer struct {
	socketServer *Server
	config       *DocConfig
	engine       *gin.Engine
	Manager      *doc.Manager
	regInfo      *regCenter.RegInfo
	moduleDoc    string
}

// doc-index --> id-list --> key list

// NewDocServer new a socket doc server
func NewDocServer(clusterId string, config *DocConfig) *DocServer {
	gin.SetMode(gin.ReleaseMode)
	e := gin.Default()
	e.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	s := &DocServer{
		config:    config,
		engine:    e,
		Manager:   doc.NewManager(),
		moduleDoc: "/docs/:md",
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
			"title": config.Doc.Title,
			"url":   s.DocUrl(),
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
	s.engine.GET("/docs/:md", func(c *gin.Context) {
		var gwUrls = make(doc.ModuleDoc)
		// 非网管模块 一直显示网关文档
		if c.Param("md") != "gateway" {
			gwUrls = s.Manager.GetModuleDocs("gateway")
		}
		c.Header("Cache-control", "private,max-age=86400")
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"prefix":  s.config.gwPrefix,
			"id":      c.Param("md"),
			"gateway": gwUrls,
			"urls":    s.Manager.GetModuleDocs(c.Param("md")),
		})
	})
}

func (s *DocServer) initDocRoute() {
	s.engine.GET(s.config.Doc.Path, func(c *gin.Context) {
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
	})
}

func (s *DocServer) initServerProxyDocRoute() {
	s.engine.GET(s.moduleDoc+"/:key", func(c *gin.Context) {
		cacheTtl := s.config.CacheTtl
		if cacheTtl <= 0 {
			cacheTtl = 600
		}
		c.Header("Cache-control", "private,max-age="+strconv.Itoa(cacheTtl))
		if addr := s.Manager.GetRandKeyDoc(c.Param("md"), c.Param("key")); addr == "" {
			c.String(404, "Sub doc url not found.")
		} else {
			pathSeg := strings.Split(addr, "://")
			schema := pathSeg[0]
			hostEnd := strings.Index(pathSeg[1], "/")
			host := pathSeg[1][0:hostEnd]
			path := pathSeg[1][hostEnd:]
			director := func(req *http.Request) {
				req.URL.Scheme = schema
				req.URL.Host = host
				req.URL.Path = path
			}
			proxy := &httputil.ReverseProxy{Director: director}
			proxy.ServeHTTP(c.Writer, c.Request)
		}
	})
}

func (s *DocServer) Start() error {
	if s.config.socketGateway {
		if err := s.initTemplate(); err != nil {
			return err
		}
		s.initIndexRoute()
		s.initServerProxyDocRoute()
	}
	s.initDocRoute()
	if err := s.engine.Run(s.config.Origin.Host.String()); err != nil {
		return err
	}
	return nil
}

func (s *DocServer) DocUrl() string {
	return s.config.Origin.String() + s.config.Doc.Path
}

func (s *DocServer) GatewayDocUrlFormat() string {
	return utils.ToStr("protocol://hostname:port", s.config.gwPrefix, s.moduleDoc)
}

func (s *DocServer) IndexDocUrl() string {
	return s.config.Origin.String() + s.moduleDoc
}

func (s *DocServer) SyncStart(cb func(error)) {
	go func() {
		if err := s.Start(); err != nil {
			cb(err)
		}
	}()
}
