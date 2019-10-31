# onlineconf-csi-driver

[Container Storage Interface](https://github.com/container-storage-interface/spec/blob/master/spec.md) driver for [OnlineConf](https://github.com/onlineconf/onlineconf) volumes.
It supposed to be used instead of `onlineconf-updater` sidecar containers in Kubernetes (or any other CSI complaint) environment to simplify deployment and drastically reduce amount of running updaters.

## Deployment

*onlineconf-csi-driver* must be deployed on every node of a cluster (see [Kubernetes CSI driver deployment guide](https://kubernetes-csi.github.io/docs/deploying.html)).
[Draft deployment manifest](./deploy.yaml) can be used as an example.

## Usage

See [example manifest](./example.yaml) for more details.
