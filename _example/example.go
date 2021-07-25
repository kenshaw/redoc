// _example/example.go
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"

	"github.com/kenshaw/diskcache"
	"github.com/kenshaw/httplog"
	"github.com/kenshaw/redoc"
)

func main() {
	verbose := flag.Bool("v", false, "enable verbose")
	addr := flag.String("l", ":9090", "listen")
	urlstr := flag.String("url", "https://petstore.swagger.io/v2/swagger.json", "swagger url")
	spec := flag.String("spec", "/v1/swagger.json", "swagger location")
	flag.Parse()
	if err := run(context.Background(), *verbose, *addr, *urlstr, *spec); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run runs the redoc server.
func run(ctx context.Context, verbose bool, addr, urlstr, spec string) error {
	// build transport
	transport, err := buildTransport(verbose)
	if err != nil {
		return err
	}
	// get swagger
	swagger, err := get(ctx, urlstr, transport)
	if err != nil {
		return err
	}
	// create mux
	mux := http.NewServeMux()
	mux.HandleFunc(spec, func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		_, _ = res.Write(swagger)
	})
	// add redoc to mux
	if err := redoc.New(spec, "/", redoc.WithServeMux(mux)).Build(ctx, transport); err != nil {
		return err
	}
	// listen and serve
	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return http.Serve(l, mux)
}

// get retrieves the url using the provided transport.
func get(ctx context.Context, urlstr string, transport http.RoundTripper) ([]byte, error) {
	req, err := http.NewRequest("GET", urlstr, nil)
	cl := &http.Client{
		Transport: transport,
	}
	res, err := cl.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

// buildTransport builds a transport for use by redoc.
func buildTransport(verbose bool) (http.RoundTripper, error) {
	// create disk cache
	cache, err := diskcache.New(
		diskcache.WithAppCacheDir("redoc-example"),
	)
	if err != nil {
		return nil, err
	}
	if !verbose {
		return cache, nil
	}
	// add logging
	return httplog.NewPrefixedRoundTripLogger(
		cache,
		fmt.Printf,
		httplog.WithReqResBody(false, false),
	), nil
}
