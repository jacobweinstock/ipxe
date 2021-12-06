[![Test and Build](https://github.com/jacobweinstock/ipxe/actions/workflows/ci.yaml/badge.svg)](https://github.com/jacobweinstock/ipxe/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/jacobweinstock/ipxe/branch/main/graph/badge.svg)](https://codecov.io/gh/jacobweinstock/ipxe)
[![Go Report Card](https://goreportcard.com/badge/github.com/jacobweinstock/ipxe)](https://goreportcard.com/report/github.com/jacobweinstock/ipxe)
[![Go Reference](https://pkg.go.dev/badge/github.com/jacobweinstock/ipxe.svg)](https://pkg.go.dev/github.com/jacobweinstock/ipxe)

# ipxe

tftp and http server for ipxe binaries

## Usage

```bash
go run cmd/ipxe/main.go
```

## Design Philosophy

This repository is designed to be both a library and a command line tool.

## Architecture

The following architectural diagram details how this iPXE binary service fits into the greater PXE stack. [Architecture](docs/architecture.png).
