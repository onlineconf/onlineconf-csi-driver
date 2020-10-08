package main

import (
	"context"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type controllerServer struct {
	csi.UnimplementedControllerServer
}

func newControllerServer() *controllerServer {
	return &controllerServer{}
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities missing in request")
	}
	for _, volCap := range req.GetVolumeCapabilities() {
		if _, err := readVolumeCapability(volCap); err != nil {
			return nil, err
		}
	}
	size := int64(0)
	if req.GetCapacityRange() != nil {
		size = req.GetCapacityRange().GetRequiredBytes()
	}
	volCtx, err := readVolumeContext(req.GetParameters())
	if err != nil {
		return nil, err
	}
	for k, v := range volCtx.vars {
		volCtx.vars[k] = os.Expand(v, func(name string) string {
			switch name {
			case "pvc.name":
				return req.GetParameters()["csi.storage.k8s.io/pvc/name"]
			case "pvc.namespace":
				return req.GetParameters()["csi.storage.k8s.io/pvc/namespace"]
			case "pv.name":
				return req.GetParameters()["csi.storage.k8s.io/pv/name"]
			default:
				return ""
			}
		})
	}
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.GetName(),
			VolumeContext: volCtx.volumeContext(),
			CapacityBytes: size,
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
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
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities missing in request")
	}
	for _, volCap := range req.GetVolumeCapabilities() {
		if _, err := readVolumeCapability(volCap); err != nil {
			return &csi.ValidateVolumeCapabilitiesResponse{Message: err.Error()}, nil
		}
	}
	volCtx, err := readVolumeContext(req.GetVolumeContext())
	if err != nil {
		return &csi.ValidateVolumeCapabilitiesResponse{Message: err.Error()}, nil
	}
	params, err := readVolumeContext(req.GetParameters())
	if err != nil {
		return &csi.ValidateVolumeCapabilitiesResponse{Message: err.Error()}, nil
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
			VolumeContext:      volCtx.volumeContext(),
			Parameters:         params.volumeContext(),
		},
	}, nil
}
