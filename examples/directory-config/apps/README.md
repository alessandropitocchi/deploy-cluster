# Custom Applications

Place your custom application YAML files in this directory.

Each file should define a `plugins.customApps` section:

```yaml
plugins:
  customApps:
    - name: my-app
      namespace: default
      manifest: |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: my-app
        spec:
          replicas: 1
          ...
```
