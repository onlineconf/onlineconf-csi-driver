package main

import (
	"bufio"
	"context"
	"os"
	"strings"
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
	id       string
	m        sync.Mutex
	state    *state
	updaters map[string]*updaterInfo
}

func newNodeServer(id string, state *state) *nodeServer {
	ns := &nodeServer{
		id:       id,
		state:    state,
		updaters: make(map[string]*updaterInfo),
	}
	return ns
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	log.Debug().Msg("NodeGetInfo")
	return &csi.NodeGetInfoResponse{NodeId: ns.id}, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	log.Debug().Msg("NodeGetCapabilities")
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

	log.Debug().Str("volume_id", volumeId).Str("staging_target_path", stage).Msg("NodeStageVolume")

	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId missing in request")
	}
	if stage == "" {
		return nil, status.Error(codes.InvalidArgument, "StagingTargetPath missing in request")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapability missing in request")
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

	if err := os.MkdirAll(stage, 0755); err != nil {
		log.Error().Err(err).Msg("failed to mkdir StagingTargetPath")
		return nil, status.Error(codes.Internal, "failed to mkdir StagingTargetPath")
	}

	state := updaterState{
		DataDir:  stage,
		URI:      req.GetVolumeContext()["uri"],
		Username: req.GetSecrets()["username"],
		Password: req.GetSecrets()["password"],
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

	log.Debug().Str("volume_id", volumeId).Str("staging_target_path", stage).Msg("NodeUnstageVolume")

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

	log.Debug().Str("volume_id", volumeId).Str("target_path", target).Str("staging_target_path", stage).Msg("NodePublishVolume")

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

	if source, err := getMountSource(target); err != nil {
		log.Error().Err(err).Msg("failed to read mountinfo")
		return nil, status.Error(codes.Internal, "failed to read mountinfo")
	} else if source != "" {
		if source == stage {
			return &csi.NodePublishVolumeResponse{}, nil
		} else {
			return nil, status.Error(codes.InvalidArgument, "incompatible StagingTargetPath")
		}
	}

	if err := os.MkdirAll(target, 0755); err != nil {
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

	log.Debug().Str("volume_id", volumeId).Str("target_path", target).Msg("NodeUnpublishVolume")

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

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *nodeServer) runUpdater(volumeId string, state updaterState, restore bool) error {
	log.Info().Str("volume_id", volumeId).Msg("starting updater")
	u := updater.NewUpdater(updater.UpdaterConfig{
		Admin: updater.AdminConfig{
			URI:      state.URI,
			Username: state.Username,
			Password: state.Password,
		},
		DataDir: state.DataDir,
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

func getMountSource(target string) (string, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Split(s.Text(), " ")
		if len(fields) < 5 {
			continue
		}
		if fields[4] == target {
			return fields[3], nil
		}
	}
	return "", s.Err()
}
