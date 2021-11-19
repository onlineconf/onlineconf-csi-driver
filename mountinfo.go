package main

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type mountInfo struct {
	id         string
	device     string
	root       string
	mountPoint string
}

type mountinfo []mountInfo

func readMountInfo() (mountinfo, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMountInfo(f)
}

func parseMountInfo(r io.Reader) (mountinfo, error) {
	mounts := []mountInfo{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		fields := strings.Split(s.Text(), " ")
		if len(fields) < 5 {
			continue
		}
		mounts = append(mounts, mountInfo{
			id:         fields[0],
			device:     fields[2],
			root:       fields[3],
			mountPoint: fields[4],
		})
	}
	return mounts, s.Err()
}

func (mounts mountinfo) getByMountPoint(mountPoint string) *mountInfo {
	for i := len(mounts) - 1; i >= 0; i-- {
		if mountPoint == mounts[i].mountPoint {
			return &mounts[i]
		}
	}
	return nil
}

func (mounts mountinfo) verifyMountSource(mount *mountInfo, source string) bool {
	sm := mounts.findByPath(source)
	return sm != nil && sm.device == mount.device && sm.getPathOnDevice(source) == mount.root
}

func (mounts mountinfo) findByPath(path string) *mountInfo {
	for i := len(mounts) - 1; i >= 0; i-- {
		if isPathWithin(path, mounts[i].mountPoint) {
			return &mounts[i]
		}
	}
	return nil
}

func (m mountInfo) getPathOnDevice(path string) string {
	if m.root == m.mountPoint {
		return path
	} else if m.root == "/" {
		return strings.TrimPrefix(path, m.mountPoint)
	} else if m.mountPoint == "/" {
		return m.root + path
	} else {
		return m.root + strings.TrimPrefix(path, m.mountPoint)
	}
}

func isPathWithin(path, mount string) bool {
	return strings.HasPrefix(path, mount) &&
		(mount == "/" || len(path) == len(mount) || path[len(mount)] == '/')
}
