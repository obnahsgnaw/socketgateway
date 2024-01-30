package httpproxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// New return an url reverse proxy
// proxyUrl is the url of to proxy, etd.http://127.0.0.1:8003
// pathPrefix is the path prefix of the target path to replaced, etd. /v1/doc/socket/tcp
// pathReplace is the path to replace to
func New(proxyUrl, pathPrefix, pathReplace string) (*httputil.ReverseProxy, error) {
	targetURL, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	// 重写Director方法来修改传入请求的URL
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path, req.URL.RawPath = replacePathPrefix(req.URL.Path, req.URL.RawPath, pathPrefix, pathReplace)
	}

	return proxy, nil
}

// 替换路径前缀
func replacePathPrefix(path, rawPath, oldPrefix, newPrefix string) (newPath, newRawPath string) {
	newPath = strings.Replace(path, oldPrefix, newPrefix, 1)
	if rawPath != "" {
		newRawPath = strings.Replace(rawPath, oldPrefix, newPrefix, 1)
	} else {
		newRawPath = newPath
	}
	return
}
