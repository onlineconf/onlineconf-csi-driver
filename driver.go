package main

import (
	"context"
	"net"
	"net/url"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type driver struct {
	server *grpc.Server
	ns     *nodeServer
}

func newDriver() *driver {
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(loggingInterceptor))
	csi.RegisterIdentityServer(server, newIdentityServer())
	return &driver{server: server}
}

func (d *driver) initControllerServer() {
	csi.RegisterControllerServer(d.server, newControllerServer())
}

func (d *driver) initNodeServer(id, stateFile string) (err error) {
	d.ns, err = newNodeServer(id, stateFile)
	if err == nil {
		csi.RegisterNodeServer(d.server, d.ns)
	}
	return
}

func (d *driver) run(endpoint string) {
	if d.ns != nil {
		d.ns.start()
		defer d.ns.stop()
	}

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

func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	log.Info().Str("method", info.FullMethod).
		Str("request", protosanitizer.StripSecrets(req).String()).
		Str("response", protosanitizer.StripSecrets(resp).String()).
		Err(err).Msg("request finished")
	return
}
