package main

import (
	"strings"
	"testing"
)

func TestGetMountSource(t *testing.T) {
	mountinfo := `22 44 0:21 / /sys rw,nosuid,nodev,noexec,relatime shared:6 - sysfs sysfs rw
23 44 0:22 / /proc rw,nosuid,nodev,noexec,relatime shared:5 - proc proc rw
24 44 0:5 / /dev rw,nosuid shared:2 - devtmpfs devtmpfs rw,size=98841448k,nr_inodes=24710362,mode=755
26 24 0:23 / /dev/shm rw,nosuid,nodev shared:3 - tmpfs tmpfs rw
44 1 253:0 / / rw,relatime shared:1 - xfs /dev/mapper/system-root rw,attr2,inode64,logbufs=8,logbsize=32k,noquota
48 44 9:0 / /boot rw,relatime shared:28 - ext4 /dev/md0 rw
52 44 253:5 / /var rw,relatime shared:29 - xfs /dev/mapper/system-var rw,attr2,inode64,logbufs=8,logbsize=32k,noquota # must not be found
49 44 253:2 / /var rw,relatime shared:29 - xfs /dev/mapper/system-var rw,attr2,inode64,logbufs=8,logbsize=32k,noquota
3927 49 253:6 /zzz /var/lib/kubelet/pods/0abd8cda-b4fc-4241-bdad-13c3777daf63/volumes/kubernetes.io~csi/xxx/mount rw,relatime shared:29 - xfs /dev/mapper/system-var rw,attr2,inode64,logbufs=8,logbsize=32k,noquota # must not be found
3926 49 253:2 /lib/kubelet/plugins/kubernetes.io/csi/pv/xxx/globalmount /var/lib/kubelet/pods/0abd8cda-b4fc-4241-bdad-13c3777daf63/volumes/kubernetes.io~csi/xxx/mount rw,relatime shared:29 - xfs /dev/mapper/system-var rw,attr2,inode64,logbufs=8,logbsize=32k,noquota
50 44 253:3 /xxx /yyy rw,relatime shared:29 - xfs /dev/mapper/system-var rw,attr2,inode64,logbufs=8,logbsize=32k,noquota
51 44 253:4 / /va rw,relatime shared:29 - xfs /dev/mapper/system-var rw,attr2,inode64,logbufs=8,logbsize=32k,noquota
`
	r := strings.NewReader(mountinfo)
	mounts, err := parseMountInfo(r)
	if err != nil {
		t.Fatal(err)
	}

	source := `/var/lib/kubelet/plugins/kubernetes.io/csi/pv/xxx/globalmount`
	target := `/var/lib/kubelet/pods/0abd8cda-b4fc-4241-bdad-13c3777daf63/volumes/kubernetes.io~csi/xxx/mount`
	sourceOnDevice := `/lib/kubelet/plugins/kubernetes.io/csi/pv/xxx/globalmount`

	mount := mounts.getByMountPoint(target)
	if mount == nil {
		t.Errorf("mount not found: %q", target)
	} else if mount.id != "3926" {
		t.Errorf("invalid mount found: %q", mount.id)
	}
	sm := mounts.findByPath(source)
	if sm == nil {
		t.Errorf("source mount not found: %q", source)
	} else if sm.id != "49" {
		t.Errorf("invalid source mount found: %q", sm.id)
	}
	sod := sm.getPathOnDevice(source)
	if sod != sourceOnDevice {
		t.Errorf("invalid source path on device: %q != %q", sod, sourceOnDevice)
	}
	if !mounts.verifyMountSource(mount, source) {
		t.Error("invalid mount source")
	}

	mnt := mounts.getByMountPoint("/")
	path := mnt.getPathOnDevice("/abc/def")
	if path != "/abc/def" {
		t.Errorf("invalid path: %q != %q", path, "/abc/def")
	}

	mnt = mounts.getByMountPoint("/yyy")
	path = mnt.getPathOnDevice("/yyy/def")
	if path != "/xxx/def" {
		t.Errorf("invalid path: %q != %q", path, "/xxx/def")
	}
}
