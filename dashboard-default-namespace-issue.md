## Dashboard: Make the default namespace configurable

### Problem

The KubeRay dashboard hardcodes the default namespace to `"default"` in `NamespaceProvider.tsx`:

```typescript
const [namespace, setNamespace] = useState("default");
```

When running the dashboard in standalone mode (outside Kubeflow), there is no namespace selector UI. The dashboard always queries the `"default"` namespace on startup. This causes RBAC errors when the apiserver's service account does not have permissions to list resources in `"default"` -- for example:

```
rayjobs.ray.io is forbidden: User "system:serviceaccount:dataplatform:kuberay-apiserver"
cannot list resource "rayjobs" in API group "ray.io" in the namespace "default"
```

Users who deploy KubeRay resources in a different namespace (e.g. `dataplatform`) have no way to configure the dashboard to target that namespace without modifying the source code.

### Proposed Solution

Add a `defaultNamespace` field to the dashboard's `RuntimeConfig`, configurable via the `DEFAULT_NAMESPACE` environment variable. The dashboard should use this value as the initial namespace instead of the hardcoded `"default"`.

The config flow already supports environment variables for other settings (e.g. `API_DOMAIN`, `RAY_API_PATH`). This change follows the same pattern:

1. Add `defaultNamespace` to `RuntimeConfig` in `config-defaults.ts` (defaults to `"default"` for backward compatibility).
2. Read `DEFAULT_NAMESPACE` / `NEXT_PUBLIC_DEFAULT_NAMESPACE` env var in `config-server.ts`.
3. Propagate the value through `fetchRuntimeConfig()` in `constants.ts`.
4. Update `NamespaceProvider.tsx` to fetch and apply the configured default namespace on mount.

### Usage

```bash
# Set the default namespace when running the dashboard
DEFAULT_NAMESPACE=dataplatform npm run dev

# Or in a Kubernetes deployment
env:
  - name: DEFAULT_NAMESPACE
    value: "dataplatform"
```

### Why not just grant ClusterRole access?

Granting cluster-wide RBAC is a valid workaround, but violates the principle of least privilege. Many organizations restrict service accounts to specific namespaces. The dashboard should support this without requiring cluster-wide permissions.
