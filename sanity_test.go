package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/kubernetes-csi/csi-test/v4/pkg/sanity"
)

func TestSanity(t *testing.T) {
	sanityTest = true
	endpoint := "unix://" + os.TempDir() + "/onlineconf-csi.sock"

	d := newDriver()
	d.initControllerServer()
	d.initNodeServer("1234567890", os.TempDir()+"/onlineconf-csi-state.json")
	go d.run(endpoint)
	defer d.stop()

	secrets := fmt.Sprintf("NodeStageVolumeSecret:\n  username: %s\n  password: %s\n",
		os.Getenv("ONLINECONF_USERNAME"), os.Getenv("ONLINECONF_PASSWORD"))
	secretsFile := os.TempDir() + "/secrets.yaml"
	ioutil.WriteFile(secretsFile, []byte(secrets), 0644)
	defer os.Remove(secretsFile)

	config := sanity.NewTestConfig()
	config.Address = endpoint
	config.SecretsFile = secretsFile
	config.TestVolumeParameters = map[string]string{
		"uri": os.Getenv("ONLINECONF_URI"),
	}
	sanity.Test(t, config)
}
