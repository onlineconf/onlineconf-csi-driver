package main

import (
	"context"
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSanity(t *testing.T) {
	endpoint := "unix://" + os.TempDir() + "/onlineconf-csi.sock"

	s, _ := openState(os.TempDir() + "/onlineconf-csi-state.json")

	d := newDriver("1234567890", s)
	csi.RegisterControllerServer(d.server, &controllerServer{})
	go d.run(endpoint)
	defer d.stop()

	config := &sanity.Config{
		TargetPath:  os.TempDir() + "/onlineconf-csi-target",
		StagingPath: os.TempDir() + "/onlineconf-csi-staging",
		Address:     endpoint,
		SecretsFile: "secrets.yaml",
		TestVolumeParameters: map[string]string{
			"uri": "http://onlineconf-admin",
		},
	}
	sanity.Test(t, config)
}

type controllerServer struct{}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	log.Debug().Str("name", req.GetName()).Msg("CreateVolume")
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities missing in request")
	}
	size := int64(0)
	if req.GetCapacityRange() != nil {
		size = req.GetCapacityRange().GetRequiredBytes()
	}
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.GetName(),
			VolumeContext: req.GetParameters(),
			CapacityBytes: size,
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	log.Debug().Str("volume_id", req.GetVolumeId()).Msg("DeleteVolume")
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	log.Debug().Msg("ControllerGetCapabilities")
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	log.Debug().Str("volume_id", req.GetVolumeId()).Msg("ValidateVolumeCapabilities")
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities missing in request")
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
					},
				},
			},
			VolumeContext: req.GetVolumeContext(),
			Parameters:    req.GetParameters(),
		},
	}, nil
}

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "SourceVolumeId missing in request")
	}
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "SnapshotId missing in request")
	}
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "CapacityRange not provided")
	}
	return nil, status.Error(codes.Unimplemented, "")
}
