// Package redoc provides a ReDoc documentation portal for OpenAPI (formerly
// swagger) definitions.
package redoc

import (
	"bytes"
	"context"
	"crypto/md5"
	_ "embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/kenshaw/webfonts"
)

// Server is a redoc server.
type Server struct {
	*http.ServeMux
	spec     string
	index    string
	template []byte
	family   string
	version  string
	prefix   string
	params   map[string]interface{}
}

// New creates a new redoc server.
func New(spec, index string, opts ...Option) *Server {
	s := &Server{
		spec:     spec,
		index:    index,
		template: DefaultTemplate,
		family:   "Montserrat:300,400,700|Roboto:300,400,700",
		version:  "next",
		prefix:   "/_/",
		params: map[string]interface{}{
			"title": "ReDoc",
		},
	}
	for _, o := range opts {
		o(s)
	}
	if s.ServeMux == nil {
		s.ServeMux = http.NewServeMux()
	}
	return s
}

// Build builds the redoc server routes.
func (s *Server) Build(ctx context.Context, transport http.RoundTripper) error {
	if transport == nil {
		transport = http.DefaultTransport
	}
	// retrieve script
	scriptURL := fmt.Sprintf("https://cdn.jsdelivr.net/npm/redoc@%s/bundles/redoc.standalone.js", s.version)
	_, script, err := get(ctx, scriptURL, transport)
	if err != nil {
		return err
	}
	// build stylesheet and routes
	stylesheet, err := s.buildFontRoutes(ctx, transport)
	if err != nil {
		return err
	}
	// handle script
	scriptPath := path.Join(s.prefix, fmt.Sprintf("%x", md5.Sum(script))[:7]) + ".js"
	s.HandleFunc(scriptPath, func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/javascript")
		_, _ = res.Write(script)
	})
	// handle stylesheet
	stylesheetPath := path.Join(s.prefix, fmt.Sprintf("%x", md5.Sum(stylesheet))[:7]) + ".css"
	s.HandleFunc(stylesheetPath, func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/css")
		_, _ = res.Write(stylesheet)
	})
	// create template
	tpl, err := template.New("index.html").Parse(string(s.template))
	if err != nil {
		return err
	}
	// build params
	params := map[string]interface{}{
		"spec":       s.spec,
		"script":     scriptPath,
		"stylesheet": stylesheetPath,
	}
	for k, v := range s.params {
		if _, ok := params[k]; !ok {
			params[k] = v
		}
	}
	// build index
	index := new(bytes.Buffer)
	if err := tpl.ExecuteTemplate(index, "index.html", params); err != nil {
		return err
	}
	// handle index
	indexBuf := index.Bytes()
	s.HandleFunc(s.index, func(res http.ResponseWriter, req *http.Request) {
		_, _ = res.Write(indexBuf)
	})
	return nil
}

// buildFontRoutes builds font paths and retrieves the font files.
func (s *Server) buildFontRoutes(ctx context.Context, transport http.RoundTripper) ([]byte, error) {
	// retrieve fonts
	fonts, err := webfonts.AllFontFaces(ctx, s.family, webfonts.WithTransport(transport))
	if err != nil {
		return nil, err
	}
	// collect stylesheet and routes
	stylesheet := new(bytes.Buffer)
	err = webfonts.BuildRoutes(s.prefix, fonts, func(_ string, buf []byte, routes []webfonts.Route) error {
		for _, route := range routes {
			// retrieve
			contentType, buf, err := get(ctx, route.URL, transport)
			if err != nil {
				return err
			}
			// add route handler
			s.HandleFunc(path.Join(s.prefix, route.Path), func(res http.ResponseWriter, req *http.Request) {
				res.Header().Set("Content-Type", contentType)
				_, _ = res.Write(buf)
			})
		}
		// append stylesheet
		_, err := stylesheet.Write(buf)
		return err
	})
	if err != nil {
		return nil, err
	}
	return stylesheet.Bytes(), nil
}

// Option is a redoc server option.
type Option func(*Server)

// WithServeMux is a redoc server option to set the mux used.
func WithServeMux(serveMux *http.ServeMux) Option {
	return func(s *Server) {
		s.ServeMux = serveMux
	}
}

// WithTemplate is a redoc server option to set the template.
func WithTemplate(template []byte) Option {
	return func(s *Server) {
		s.template = template
	}
}

// WithFamily is a redoc server option to set the google webfonts family to
// use.
func WithFamily(family string) Option {
	return func(s *Server) {
		s.family = family
	}
}

// WithVersion is a redoc server option to set the redoc script version
// retrieved.
func WithVersion(version string) Option {
	return func(s *Server) {
		s.version = version
	}
}

// WithPrefix is a redoc server option to set the asset prefix.
func WithPrefix(prefix string) Option {
	return func(s *Server) {
		s.prefix = prefix
	}
}

// WithParam is a redoc server option to set an additional template param.
func WithParam(key string, value interface{}) Option {
	return func(s *Server) {
		s.params[key] = value
	}
}

// WithTitle is a redoc server option to set the redoc page title.
func WithTitle(title string) Option {
	return WithParam("title", title)
}

// get retrieves a url using the transport.
func get(ctx context.Context, urlstr string, transport http.RoundTripper) (string, []byte, error) {
	// request
	req, err := http.NewRequest("GET", urlstr, nil)
	if err != nil {
		return "", nil, err
	}
	cl := &http.Client{
		Transport: transport,
	}
	// execute
	res, err := cl.Do(req.WithContext(ctx))
	if err != nil {
		return "", nil, err
	}
	defer res.Body.Close()
	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", nil, err
	}
	return res.Header.Get("Content-Type"), buf, nil
}

// DefaultTemplate is the default index template.
//
//go:embed index.html
var DefaultTemplate []byte
