[![CI](https://github.com/aqua/kasadutycycle/actions/workflows/go.yml/badge.svg)](https://github.com/aqua/kasadutycycle/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/aqua/kasadutycycle)](https://goreportcard.com/report/github.com/aqua/kasadutycycle)
[![Go Reference](https://pkg.go.dev/badge/github.com/aqua/kasadutycycle.svg)](https://pkg.go.dev/github.com/aqua/kasadutycycle)

A Prometheus exporter for Kasa/TPLink smart plugs.  Oriented around the
need to estimate duty cycles for appliances like refrigerators and freezers,
where limited downstream processing of the prometheus metrics may be feasible.

For a more general-purpose monitoring of Kasa plugs using the TP-Link Smart
Home Protocol, see github.com/fffonion/tplink-plug-exporter (which this
implementation uses.)
