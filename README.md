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
  * `nodePublishSecretRef` - optional, recommended: the same secret reference. When set, every pod publish delivers the secret to the driver, so a rotated secret takes effect as soon as the next pod using the volume starts on the node (see [Credentials rotation](#credentials-rotation))
  * `readOnly`: `true` (OnlineConf volumes are always read only)
  * `volumeAttributes`:
    * `uri` - URI of *onlineconf-admin* instance
    * `datacenter` - datacenter name (default: empty)
    * `updateInterval` - polling interval for requests to *onlineconf-admin* instance (default: "10s")
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
  * `csi.storage.k8s.io/node-publish-secret-name` / `csi.storage.k8s.io/node-publish-secret-namespace` - optional, recommended: the same values as their node-stage counterparts, so a rotated secret takes effect as soon as the next pod starts (see [Credentials rotation](#credentials-rotation)).
  * `uri` - URI of *onlineconf-admin* instance
  * `datacenter` - datacenter name (default: empty)
  * `updateInterval` - polling interval for requests to *onlineconf-admin* instance (default: "10s")
  * `${any_variable_name}` - any variables you want to interpolate into OnlineConf template values. Can contain template variables `${pvc.name}`, `${pvc.namespace}` and `${pv.name}` (see docs on `csi.storage.k8s.io/node-stage-secret-name` for more details).

## Credentials rotation

The driver reads the secret through the CSI calls kubelet makes, so a change
of the Kubernetes secret alone does not reach an already staged volume —
Kubernetes never pushes secret updates to CSI drivers. Instead, whenever a
`NodeStageVolume` or `NodePublishVolume` call delivers credentials that
differ from the ones the volume's updater is running with, the driver
restarts the updater with the new credentials and persists them in its state
file. The served files are untouched during the swap, so running pods see no
interruption.

What triggers such a call after you update the secret:

* a new pod using the volume starts on the node — but only if the volume
  also carries `nodePublishSecretRef` (or the node-publish StorageClass
  parameters), which is why setting them to the same secret is recommended;
* kubelet re-stages already mounted volumes after a restart, delivering
  freshly read `nodeStageSecretRef` secrets;
* the volume is fully unstaged and staged again (the last pod using it
  leaves the node and a new one arrives).

If the new credentials are refused by *onlineconf-admin*, the call fails,
the previous updater keeps running (serving the already fetched
configuration and retrying on its usual interval) and the rotation is
retried on the next stage or publish call, so a mistyped secret cannot
silently freeze the volume.

Rotate gracefully: create the new account in OnlineConf first, then update
the Kubernetes secret, and remove the old account only after the driver
logs `rotating credentials` for the affected volumes (or after the old
account stops appearing in *onlineconf-admin* access logs).
