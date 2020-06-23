# onlineconf-csi-driver

[Container Storage Interface](https://github.com/container-storage-interface/spec/blob/master/spec.md) driver for [OnlineConf](https://github.com/onlineconf/onlineconf) volumes.
It supposed to be used instead of `onlineconf-updater` sidecar containers in Kubernetes (or any other CSI complaint) environment to simplify deployment and drastically reduce amount of running updaters.

## Deployment

*onlineconf-csi-driver* must be deployed on every node of a cluster (see [Kubernetes CSI driver deployment guide](https://kubernetes-csi.github.io/docs/deploying.html)).
[Draft deployment manifest](./deploy.yaml) can be used as an example.

## Usage

See [example manifest](./example.yaml) for more details.

### Persistent Volume configuration

* `accessModes` - must be `ReadOnlyMany`
* `capacity` - not used by *onlineconf-csi-driver* right now. This field is required by Kubernetes, should be set to something reasonable.
* `csi`:
  * `driver`: `csi.onlineconf.mail.ru`
  * `nodeStageSecretRef` - a reference to a secret containing `username` and `password` used to authenticate in *onlineconf-admin*
  * `readOnly`: `true` (OnlineConf volumes are always read only)
  * `volumeAttributes`:
    * `uri` - URI of *onlineconf-admin* instance
    * `${any_variable_name}` - any variables you want to interpolate into OnlineConf template values
  * `volumeHandle` - required by Kubernetes
* `mountOptions` - optional, supported options:
  * `mode=` - file mode bits of the volume root directory (default: `750`)
* `volumeMode` - optional, must be `Filesystem` (default)
