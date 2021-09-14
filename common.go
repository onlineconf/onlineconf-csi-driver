package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var sanityTest bool

type volumeCapability struct {
	chmod bool
	mode  os.FileMode
}

func readVolumeCapability(capability *csi.VolumeCapability) (*volumeCapability, error) {
	if capability == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapability missing in request")
	}

	if mode := capability.GetAccessMode().GetMode(); mode != csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY &&
		mode != csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY &&
		!(sanityTest && mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER) {
		return nil, status.Error(codes.InvalidArgument, "unsupported access mode")
	}

	mount := capability.GetMount()
	if mount == nil {
		return nil, status.Error(codes.InvalidArgument, "AccessType must be mount")
	}
	if mount.GetFsType() != "" && mount.GetFsType() != "ext4" { // ext4 is set by external-provisioner
		return nil, status.Error(codes.InvalidArgument, "unsupported filesystem type")
	}

	cap := &volumeCapability{}
	for _, flag := range mount.GetMountFlags() {
		if strings.HasPrefix(flag, "mode=") {
			val, err := strconv.ParseUint(flag[5:], 8, 12)
			if err != nil {
				log.Error().Err(err).Msg("failed to parse mode")
				return nil, status.Error(codes.InvalidArgument, "invalid mount flags")
			}
			cap.chmod = true
			cap.mode = os.FileMode(val)
		}
	}
	return cap, nil
}

type volumeContext struct {
	uri            string
	updateInterval time.Duration
	vars           map[string]string
}

func readVolumeContext(parameters map[string]string) (*volumeContext, error) {
	ctx := &volumeContext{
		uri:  parameters["uri"],
		vars: make(map[string]string, len(parameters)),
	}

	if ctx.uri == "" {
		return nil, status.Error(codes.InvalidArgument, "uri is required")
	}

	if intervalStr := parameters["updateInterval"]; intervalStr != "" {
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("updateInterval invalid value: %v", err))
		}
		ctx.updateInterval = interval
	}

	for k, v := range parameters {
		if strings.HasPrefix(k, "${") && strings.HasSuffix(k, "}") {
			ctx.vars[k[2:len(k)-1]] = v
		}
	}
	return ctx, nil
}

func (volCtx *volumeContext) volumeContext() map[string]string {
	volumeContext := map[string]string{"uri": volCtx.uri}
	for k, v := range volCtx.vars {
		volumeContext["${"+k+"}"] = v
	}
	return volumeContext
}
