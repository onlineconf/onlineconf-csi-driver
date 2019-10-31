package main

import (
	"net"
	"net/url"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"google.golang.org/grpc"

	"github.com/rs/zerolog/log"
)

type driver struct {
	server *grpc.Server
	ns     *nodeServer
}

func newDriver(nodeId string, state *state) *driver {
	ids := newIdentityServer()
	ns := newNodeServer(nodeId, state)

	server := grpc.NewServer()
	csi.RegisterIdentityServer(server, ids)
	csi.RegisterNodeServer(server, ns)

	return &driver{server, ns}
}

func (d *driver) run(endpoint string) {
	d.ns.start()
	defer d.ns.stop()

	uri, err := url.Parse(endpoint)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse endpoint")
	}

	if uri.Scheme == "unix" {
		if err := os.Remove(uri.Path); err != nil && !os.IsNotExist(err) {
			log.Fatal().Err(err).Str("addr", uri.Path).Msg("failed to remove socket")
		}
	}

	listener, err := net.Listen(uri.Scheme, uri.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}

	err = d.server.Serve(listener)
	if err != nil {
		log.Fatal().Err(err).Msg("filed to serve")
	}
}

func (d *driver) stop() {
	d.server.Stop()
}
