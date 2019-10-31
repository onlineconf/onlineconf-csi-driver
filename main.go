package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
)

var (
	endpoint  = flag.String("endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	nodeId    = flag.String("node", "", "node id")
	stateFile = flag.String("state", "/var/lib/onlineconf-csi-driver/state.json", "state file")
)

func main() {
	flag.Parse()

	state, err := openState(*stateFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open state file")
	}

	driver := newDriver(*nodeId, state)

	log.Info().Msg("onlineconf-csi-driver started")
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigC)
	go func() {
		sig := <-sigC
		log.Info().Str("signal", sig.String()).Msg("signal received, terminating")
		signal.Stop(sigC)
		driver.stop()
	}()

	driver.run(*endpoint)
	log.Info().Msg("onlineconf-csi-driver stopped")
}
