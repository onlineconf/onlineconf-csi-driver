package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/onlineconf/onlineconf/updater/v3/updater"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type updaterInfo struct {
	updater *updater.Updater
	wg      sync.WaitGroup
}

type nodeServer struct {
	csi.UnimplementedNodeServer
	id       string
	m        sync.Mutex
	state    *state
	updaters map[string]*updaterInfo
}

func newNodeServer(id string, stateFile string) (*nodeServer, error) {
	state, err := readState(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open state file: %w", err)
	}
	return &nodeServer{
		id:       id,
		state:    state,
		updaters: make(map[string]*updaterInfo),
	}, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{NodeId: ns.id}, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	stage := req.GetStagingTargetPath()

	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if stage == "" {
		return nil, status.Error(codes.InvalidArgument, "StagingTargetPath missing in request")
	}

	volCap, err := readVolumeCapability(req.GetVolumeCapability())
	if err != nil {
		return nil, err
	}
	volCtx, err := readVolumeContext(req.GetVolumeContext())
	if err != nil {
		return nil, err
	}

	ns.m.Lock()
	defer ns.m.Unlock()

	if us, ok := ns.state.Updaters[volumeId]; ok {
		if us.DataDir == stage {
			return &csi.NodeStageVolumeResponse{}, nil
		} else {
			return nil, status.Error(codes.InvalidArgument, "volume is already staged to another StagingTargetPath")
		}
	}

	if _, ok := ns.updaters[stage]; ok {
		return nil, status.Error(codes.InvalidArgument, "another volume is already staged to requested StagingTargetPath")
	}

	if err := os.MkdirAll(stage, 0750); err != nil {
		log.Error().Err(err).Msg("failed to mkdir StagingTargetPath")
		return nil, status.Error(codes.Internal, "failed to mkdir StagingTargetPath")
	}

	if volCap.chmod {
		if err := os.Chmod(stage, volCap.mode); err != nil {
			log.Error().Err(err).Msg("failed to chmod StagingTargetPath")
			return nil, status.Error(codes.Internal, "failed to chmod StagingTargetPath")
		}
	}

	state := updaterState{
		DataDir:        stage,
		URI:            volCtx.uri,
		Username:       req.GetSecrets()["username"],
		Password:       req.GetSecrets()["password"],
		UpdateInterval: volCtx.updateInterval,
		Variables:      volCtx.vars,
	}
	if err := ns.runUpdater(volumeId, state, false); err != nil {
		log.Error().Err(err).Msg("failed to run updater")
		return nil, status.Error(codes.Internal, err.Error())
	}
	ns.state.Updaters[volumeId] = state
	ns.state.save()

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	stage := req.GetStagingTargetPath()

	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if stage == "" {
		return nil, status.Error(codes.InvalidArgument, "StagingTargetPath missing in request")
	}

	ns.m.Lock()
	defer ns.m.Unlock()

	if us, ok := ns.state.Updaters[volumeId]; !(ok && us.DataDir == stage) {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if ui := ns.updaters[stage]; ui != nil {
		log.Info().Str("volume_id", volumeId).Msg("stopping updater")
		ui.updater.Stop()
		ui.wg.Wait()
	}

	if err := os.RemoveAll(stage); err != nil {
		log.Error().Err(err).Msg("failed to remove StagingTargetDir")
		return nil, status.Error(codes.Internal, "failed to remove StagingTargetPath")
	}
	delete(ns.state.Updaters, volumeId)
	ns.state.save()

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	target := req.GetTargetPath()
	stage := req.GetStagingTargetPath()

	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if target == "" {
		return nil, status.Error(codes.InvalidArgument, "TargetPath missing in request")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapability missing in request")
	}
	if stage == "" {
		return nil, status.Error(codes.FailedPrecondition, "StagingTargetPath missing in request")
	}

	ns.m.Lock()
	defer ns.m.Unlock()

	if us, ok := ns.state.Updaters[volumeId]; !ok {
		return nil, status.Error(codes.NotFound, "unknown VolumeId")
	} else if us.DataDir != stage {
		return nil, status.Error(codes.InvalidArgument, "incompatible VolumeId and StagingTargetPath")
	}

	if mounts, err := readMountInfo(); err != nil {
		log.Error().Err(err).Msg("failed to read mountinfo")
		return nil, status.Error(codes.Internal, "failed to read mountinfo")
	} else if mount := mounts.getByMountPoint(target); mount != nil {
		if mounts.verifyMountSource(mount, stage) {
			return &csi.NodePublishVolumeResponse{}, nil
		} else {
			return nil, status.Error(codes.InvalidArgument, "incompatible StagingTargetPath")
		}
	}

	if err := os.MkdirAll(target, 0750); err != nil {
		log.Error().Err(err).Msg("failed to mkdir")
		return nil, status.Error(codes.Internal, "failed to mkdir TargetPath")
	}

	if err := syscall.Mount(stage, target, "", syscall.MS_MGC_VAL|syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		log.Error().Err(err).Msg("failed to mount")
		return nil, status.Error(codes.Internal, "failed to mount")
	}

	if err := syscall.Mount(stage, target, "", syscall.MS_MGC_VAL|syscall.MS_REMOUNT|syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		log.Error().Err(err).Msg("failed to remount")
		return nil, status.Error(codes.Internal, "failed to remount")
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	target := req.GetTargetPath()

	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if target == "" {
		return nil, status.Error(codes.InvalidArgument, "TargetPath missing in request")
	}

	ns.m.Lock()
	defer ns.m.Unlock()

	if err := syscall.Unmount(target, 0); err != nil && err != syscall.EINVAL && err != syscall.ENOENT {
		log.Error().Err(err).Msg("failed to unmount")
		return nil, status.Error(codes.Internal, "failed to unmount")
	}

	if err := os.RemoveAll(target); err != nil {
		log.Error().Err(err).Msg("failed to remove TargetPath")
		return nil, status.Error(codes.Internal, "failed to remove TargetPath")
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) runUpdater(volumeId string, state updaterState, restore bool) error {
	log.Info().Str("volume_id", volumeId).Dur("updateInterval", state.UpdateInterval).Msg("starting updater")

	u := updater.NewUpdater(updater.UpdaterConfig{
		Admin: updater.AdminConfig{
			URI:      state.URI,
			Username: state.Username,
			Password: state.Password,
		},
		UpdateInterval: state.UpdateInterval,
		DataDir:        state.DataDir,
		Variables:      state.Variables,
	})
	if err := u.Update(); err != nil {
		if !restore {
			return err
		}
		log.Error().Err(err).Str("volume_id", volumeId).Msg("update failed")
	}

	ui := &updaterInfo{updater: u}
	ui.wg.Add(1)
	ns.updaters[state.DataDir] = ui
	log.Info().Str("volume_id", volumeId).Msg("updater started")
	go func() {
		u.Run()
		log.Info().Str("volume_id", volumeId).Msg("updater stopped")
		delete(ns.updaters, state.DataDir)
		ui.wg.Done()
	}()

	return nil
}

func (ns *nodeServer) start() {
	ns.m.Lock()
	defer ns.m.Unlock()

	for volumeId, state := range ns.state.Updaters {
		_, err := os.Stat(state.DataDir)
		if err != nil {
			continue
		}

		ns.runUpdater(volumeId, state, true)
	}
}

func (ns *nodeServer) stop() {
	ns.m.Lock()
	defer ns.m.Unlock()

	for _, ui := range ns.updaters {
		ui.updater.Stop()
	}
	for _, ui := range ns.updaters {
		ui.wg.Wait()
	}
}
