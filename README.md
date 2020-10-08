# onlineconf-csi-driver

[Container Storage Interface](https://github.com/container-storage-interface/spec/blob/master/spec.md) driver for [OnlineConf](https://github.com/onlineconf/onlineconf) volumes.
It supposed to be used instead of `onlineconf-updater` sidecar containers in Kubernetes (or any other CSI complaint) environment to simplify deployment and drastically reduce amount of running updaters.

## Deployment

*onlineconf-csi-driver* must be deployed on every node of a cluster (see [Kubernetes CSI driver deployment guide](https://kubernetes-csi.github.io/docs/deploying.html)).
Additionally, for dynamic provisioning to work, exactly one instance of *onlineconf-csi-driver* working in controller mode is required.
[Draft deployment manifest](./deploy.yaml) can be used as an example.

## Usage

The driver supports both static and dynamic volume provisioning.

### Static volume provisioning

See [example manifest (static)](./example.yaml) for more details.

#### Persistent Volume configuration

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

### Dynamic volume provisioning

See [example manifest (dynamic)](./example-dynamic.yaml) for more details.

#### Storage Class configuration

* `provisioner` - must be `csi.onlineconf.mail.ru`
* `mountOptions` - optional, supported options:
  * `mode=` - file mode bits of the volume root directory (default: `750`)
* `parameters`:
  * `csi.storage.k8s.io/node-stage-secret-name` - a name of a secret containing `username` and `password` used to authenticate in *onlineconf-admin*. Can contain template variables `${pvc.name}`, `${pvc.namespace}`, `${pv.name}` and `${pvc.annotations['<ANNOTATION_KEY>']}`, see [Kubernetes CSI docs](https://kubernetes-csi.github.io/docs/secrets-and-credentials-storage-class.html#node-stage-secret) for more information. Recommended value is `${pvc.name}`.
  * `csi.storage.k8s.io/node-stage-secret-namespace` - a namespace of this secret. Can contain template variables `${pvc.namespace}` and `${pv.name}`. Recommended value is `${pvc.namespace}`.
  * `uri` - URI of *onlineconf-admin* instance
  * `${any_variable_name}` - any variables you want to interpolate into OnlineConf template values. Can contain template variables `${pvc.name}`, `${pvc.namespace}` and `${pv.name}` (see docs on `csi.storage.k8s.io/node-stage-secret-name` for more details).
