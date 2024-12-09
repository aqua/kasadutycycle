package exporter

import (
	"net/http"

	// "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HttpServer struct {
	mux *http.ServeMux
}

func (e *Exporter) NewHttpServer() *HttpServer {
	s := &HttpServer{
		mux: http.NewServeMux(),
	}
	e.registry.MustRegister(e)
	s.mux.HandleFunc("/metrics", promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{}).ServeHTTP)
	return s
}

func (s *HttpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
