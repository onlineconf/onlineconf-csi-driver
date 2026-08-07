package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func TestCredentialsChanged(t *testing.T) {
	us := updaterState{Username: "alice", Password: "a1"}
	tests := []struct {
		name    string
		secrets map[string]string
		want    bool
	}{
		{"same credentials", map[string]string{"username": "alice", "password": "a1"}, false},
		{"new password", map[string]string{"username": "alice", "password": "a2"}, true},
		{"new account", map[string]string{"username": "bob", "password": "b1"}, true},
		{"no secrets attached to the call", map[string]string{}, false},
		{"nil secrets", nil, false},
		{"unrelated secret keys only", map[string]string{"token": "x"}, false},
		{"explicit empty credentials differ", map[string]string{"username": "", "password": ""}, true},
	}
	for _, tt := range tests {
		if got := credentialsChanged(us, tt.secrets); got != tt.want {
			t.Errorf("%s: credentialsChanged(%v) = %v, want %v", tt.name, tt.secrets, got, tt.want)
		}
	}
}

// adminStub is a minimal /client/config endpoint: it records the last basic
// auth pair it saw and serves an empty, valid CBOR config — enough for the
// embedded updater's fetch to succeed.
type adminStub struct {
	mu       sync.Mutex
	lastUser string
	lastPass string
	denied   map[string]bool
}

// {"modules": [], "nodes": []} in CBOR
var emptyConfigCBOR = []byte{0xa2, 0x67, 'm', 'o', 'd', 'u', 'l', 'e', 's', 0x80, 0x65, 'n', 'o', 'd', 'e', 's', 0x80}

func (s *adminStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, pass, _ := r.BasicAuth()
	s.mu.Lock()
	s.lastUser, s.lastPass = user, pass
	denied := s.denied[user]
	s.mu.Unlock()
	if denied {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.Header().Set("X-OnlineConf-Admin-Last-Modified", "2026-01-01 00:00:00")
	w.Write(emptyConfigCBOR)
}

func (s *adminStub) last() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastUser, s.lastPass
}

func readerOnlyMountCapability() *csi.VolumeCapability {
	return &csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY},
		AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
	}
}

func stageRequest(volumeId, stage, uri string, secrets map[string]string) *csi.NodeStageVolumeRequest {
	return &csi.NodeStageVolumeRequest{
		VolumeId:          volumeId,
		StagingTargetPath: stage,
		VolumeCapability:  readerOnlyMountCapability(),
		VolumeContext:     map[string]string{"uri": uri},
		Secrets:           secrets,
	}
}

func TestCredentialsRotation(t *testing.T) {
	stub := &adminStub{denied: map[string]bool{"mallory": true}}
	server := httptest.NewServer(stub)
	defer server.Close()

	dir := t.TempDir()
	stage := filepath.Join(dir, "stage")
	stateFile := filepath.Join(dir, "state.json")
	ns, err := newNodeServer("test-node", stateFile)
	if err != nil {
		t.Fatal(err)
	}
	defer ns.stop()
	ctx := context.Background()

	// initial stage fetches with the initial credentials
	if _, err := ns.NodeStageVolume(ctx, stageRequest("vol1", stage, server.URL,
		map[string]string{"username": "alice", "password": "a1"})); err != nil {
		t.Fatalf("stage: %v", err)
	}
	if user, pass := stub.last(); user != "alice" || pass != "a1" {
		t.Fatalf("initial fetch as %s/%s, want alice/a1", user, pass)
	}

	// a re-stage with unchanged secrets must not restart the updater
	before := ns.updaters[stage]
	if _, err := ns.NodeStageVolume(ctx, stageRequest("vol1", stage, server.URL,
		map[string]string{"username": "alice", "password": "a1"})); err != nil {
		t.Fatalf("idempotent stage: %v", err)
	}
	if ns.updaters[stage] != before {
		t.Fatal("unchanged secrets restarted the updater")
	}

	// a re-stage with new secrets rotates: the fetch runs as the new account
	// and both the in-memory and the on-disk state carry it
	if _, err := ns.NodeStageVolume(ctx, stageRequest("vol1", stage, server.URL,
		map[string]string{"username": "bob", "password": "b1"})); err != nil {
		t.Fatalf("rotating stage: %v", err)
	}
	if user, pass := stub.last(); user != "bob" || pass != "b1" {
		t.Fatalf("rotated fetch as %s/%s, want bob/b1", user, pass)
	}
	if us := ns.state.Updaters["vol1"]; us.Username != "bob" || us.Password != "b1" {
		t.Fatalf("state after rotation: %s/%s, want bob/b1", us.Username, us.Password)
	}
	if persisted, err := readState(stateFile); err != nil || persisted.Updaters["vol1"].Username != "bob" {
		t.Fatalf("persisted state after rotation: %+v, %v", persisted.Updaters["vol1"], err)
	}

	// publish with nodePublishSecretRef secrets rotates too — before any
	// mount handling, so it takes effect even where the bind mount itself
	// needs privileges this test may not have
	target := filepath.Join(dir, "target")
	if _, err := ns.NodePublishVolume(ctx, &csi.NodePublishVolumeRequest{
		VolumeId:          "vol1",
		TargetPath:        target,
		StagingTargetPath: stage,
		VolumeCapability:  readerOnlyMountCapability(),
		Secrets:           map[string]string{"username": "carol", "password": "c1"},
	}); err == nil {
		defer syscall.Unmount(target, 0)
	}
	if us := ns.state.Updaters["vol1"]; us.Username != "carol" {
		t.Fatalf("publish did not rotate: state carries %s, want carol", us.Username)
	}

	// publish without secrets must not touch the credentials
	ns.NodePublishVolume(ctx, &csi.NodePublishVolumeRequest{
		VolumeId:          "vol1",
		TargetPath:        target,
		StagingTargetPath: stage,
		VolumeCapability:  readerOnlyMountCapability(),
	})
	if us := ns.state.Updaters["vol1"]; us.Username != "carol" {
		t.Fatalf("secretless publish changed credentials to %s", us.Username)
	}

	// refused credentials fail the call, keep the previous state and bring
	// the previous updater back, so config keeps being served and the next
	// call retries the rotation
	if _, err := ns.NodeStageVolume(ctx, stageRequest("vol1", stage, server.URL,
		map[string]string{"username": "mallory", "password": "m1"})); err == nil {
		t.Fatal("rotation to refused credentials must fail the call")
	}
	if us := ns.state.Updaters["vol1"]; us.Username != "carol" {
		t.Fatalf("failed rotation leaked into state: %s", us.Username)
	}
	if ns.updaters[stage] == nil {
		t.Fatal("the previous updater was not restored after a failed rotation")
	}

	// and the retry with working credentials succeeds
	if _, err := ns.NodeStageVolume(ctx, stageRequest("vol1", stage, server.URL,
		map[string]string{"username": "dave", "password": "d1"})); err != nil {
		t.Fatalf("rotation retry: %v", err)
	}
	if us := ns.state.Updaters["vol1"]; us.Username != "dave" {
		t.Fatalf("state after retry: %s, want dave", us.Username)
	}
}
