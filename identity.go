package main

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/rs/zerolog/log"
)

type identityServer struct{}

func newIdentityServer() *identityServer {
	return &identityServer{}
}

func (ids *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	log.Debug().Msg("GetPluginInfo")
	return &csi.GetPluginInfoResponse{
		Name:          "csi.onlineconf.mail.ru",
		VendorVersion: version,
	}, nil
}

func (ids *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	log.Debug().Msg("GetPluginCapabilities")
	return &csi.GetPluginCapabilitiesResponse{Capabilities: []*csi.PluginCapability{}}, nil
}

func (ids *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	log.Debug().Msg("Probe")
	return &csi.ProbeResponse{}, nil
}
