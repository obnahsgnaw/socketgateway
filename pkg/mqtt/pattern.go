package mqtt

import (
	"github.com/obnahsgnaw/goutils/strutil"
	"regexp"
)

// ParamPattern parse {param} in regexp pattern
type ParamPattern struct {
	re *regexp.Regexp
}

// NewParamPattern return a ParamPattern
func NewParamPattern(pattern string) (*ParamPattern, error) {
	re, err := regexp.Compile(convertPattern(pattern))
	if err != nil {
		return nil, err
	}
	return &ParamPattern{re: re}, nil
}

// Parse the str params
func (p *ParamPattern) Parse(str string) map[string]string {
	result := make(map[string]string)
	matches := p.re.FindStringSubmatch(str)
	if matches != nil {
		names := p.re.SubexpNames()
		for i, name := range names {
			if i > 0 && i < len(matches) && name != "" {
				result[name] = matches[i]
			}
		}
	}
	return result
}

// convertPattern 将/{param}格式的路径转换为正则表达式
func convertPattern(pattern string) string {
	re := regexp.MustCompile(`{(\w+)}`)
	converted := re.ReplaceAllStringFunc(pattern, func(param string) string {
		name := param[1 : len(param)-1]
		return strutil.ToString("(?P<", name, ">[^/]+)")
	})
	return strutil.ToString("^", converted, "$")
}
