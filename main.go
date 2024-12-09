package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/aqua/kasadutycycle/collector"
	"github.com/aqua/kasadutycycle/exporter"
	"github.com/jonboulle/clockwork"
)

type targetList []string

func (l *targetList) String() string {
	return fmt.Sprint(*l)
}
func (l *targetList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

var (
	targetsFlag       targetList
	httpListenAddress = flag.String("http-listen-address", "localhost:8080", "Address for Prometheus HTTP server ([address]:port)")
	checkpointFile    = flag.String("checkpoint-file", "", "Path to save checkpoints (preserves continuity across restarts)")
)

func init() {
	flag.Var(&targetsFlag, "targets", "Target(s) to monitor")
}

func main() {
	flag.Parse()
	c := collector.New(targetsFlag, *checkpointFile, clockwork.NewRealClock())
	e := exporter.New(c)
	s := make(chan bool)
	// shutdown on s
	go c.Run(s)
	srv := e.NewHttpServer()
	log.Printf("will listen on %s", *httpListenAddress)
	log.Fatal(http.ListenAndServe(*httpListenAddress, srv))
}
