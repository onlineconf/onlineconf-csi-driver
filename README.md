# onlineconf-csi-driver

[Container Storage Interface](https://github.com/container-storage-interface/spec/blob/master/spec.md) driver for [OnlineConf](https://github.com/onlineconf/onlineconf) volumes.
It supposed to be used instead of `onlineconf-updater` sidecar containers in Kubernetes (or any other CSI complaint) environment to simplify deployment and drastically reduce amount of running updaters.
