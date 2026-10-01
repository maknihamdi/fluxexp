## 1. Close the in-cluster context gap

- [ ] 1.1 In `internal/k8s/contexts.go`, add an overridable `inClusterConfig = rest.InClusterConfig` package var (same pattern as the overridable `now` in `internal/resolver/freshness.go`) so both branches are testable without a cluster
- [ ] 1.2 Export the sentinel name as a constant (`k8s.InClusterContext = "in-cluster"`) — it is read by both `contexts.go` and `internal/ui/service.go`, so it must not be a literal in two files
- [ ] 1.3 In `ListContexts`, when the loaded kubeconfig yields no contexts and `inClusterConfig()` succeeds, return exactly that one entry: name the sentinel, `Current: true`, `Cluster` set to the API server host from the returned config
- [ ] 1.4 When the kubeconfig yields no contexts and `inClusterConfig()` fails, return an error naming both missing paths — today it returns an empty list and no error, which is what leaves the selector blank with nothing explaining it
- [ ] 1.5 Leave the normal path untouched: a kubeconfig with contexts must behave exactly as before, in-cluster credentials or not
- [ ] 1.6 In `internal/ui/service.go`, map the sentinel to `""` in `clientFor` before calling `newClient` — `LoadClient("", "")` is what resolves in-cluster; passing the sentinel as a context override fails with "context was not found" and never reaches the fallback
- [ ] 1.7 In `internal/ui/web/app.js` `loadContexts()`, label the option from the context's cluster when its name is the sentinel, so the bar reads as the cluster rather than as a magic word

## 2. Tests for that gap

- [ ] 2.1 `contexts_test.go`: with the existing kubeconfig fixture, the context list is unchanged whether or not `inClusterConfig` is stubbed to succeed
- [ ] 2.2 With no kubeconfig and `inClusterConfig` stubbed to succeed, exactly one context is returned, named the sentinel, marked current, carrying the stubbed host
- [ ] 2.3 With no kubeconfig and `inClusterConfig` stubbed to fail, an error is returned naming both paths
- [ ] 2.4 `internal/ui/service_test.go`: requesting the sentinel context calls the client factory with an empty context name
- [ ] 2.5 `make vet` and `make test` green

## 3. The kustomize base

- [ ] 3.1 Create `deploy/` with `kustomization.yaml`: `namespace: fluxexp`, one `labels:` entry with `includeSelectors: true`, and the resource list
- [ ] 3.2 `namespace.yaml`, `serviceaccount.yaml`
- [ ] 3.3 `rbac.yaml`: ClusterRole with **two separate rules** — `["*"]/["*"]` minus secrets with `get,list`, then a second rule for `secrets` alone — plus a comment on the Secret rule stating what deleting it costs (HelmRelease expansion) and why it cannot be narrowed (`resourceNames` does not apply to `list`); ClusterRoleBinding to the ServiceAccount
- [ ] 3.4 `deployment.yaml`: **no `replicas`**, image pinned to the released tag, args `["ui", "--address", "0.0.0.0:8765"]`, container port named `http`
- [ ] 3.5 Same file: `securityContext` (runAsNonRoot, runAsUser 65532, readOnlyRootFilesystem, allowPrivilegeEscalation false, drop ALL, seccompProfile RuntimeDefault), no volumes
- [ ] 3.6 Same file: `resources` with `requests` cpu `100m` / memory `128Mi`, `limits` memory `256Mi` and **no cpu limit**, with a comment naming the reason (CFS throttling on a bursty request path)
- [ ] 3.7 Same file: readiness and liveness `httpGet: /` on the `http` port, and `topologySpreadConstraints` over `kubernetes.io/hostname` with `whenUnsatisfiable: ScheduleAnyway`
- [ ] 3.8 `service.yaml`: ClusterIP, port 8765 named `http` targeting the named container port
- [ ] 3.9 `pdb.yaml`: `minAvailable: 1`, with a comment tying it to the HPA minimum
- [ ] 3.10 `hpa.yaml`: `autoscaling/v2`, `minReplicas: 2`, `maxReplicas: 6`, CPU `averageUtilization: 70`, with a comment that the bound is about API-server pressure and that CPU is a compromise
- [ ] 3.11 `kubectl kustomize deploy/` renders without error, and the rendered Deployment, Service and PDB selectors all match

## 4. Verify against a real cluster

- [ ] 4.1 `kubectl apply -k deploy/ --dry-run=server` on the dev cluster: schemas, RBAC and the HPA `scaleTargetRef` all accepted
- [ ] 4.2 Apply for real; confirm the HPA takes the Deployment to 2 replicas although the manifest declares no count
- [ ] 4.3 `kubectl port-forward svc/fluxexp 8765:8765` and confirm the context bar shows the single in-cluster context, not an empty selector
- [ ] 4.4 Expand a Kustomization and confirm an operator CRD in its inventory resolves with a health status rather than a permission error
- [ ] 4.5 Expand a **HelmRelease** — this is the test of the Secret rule; if it lists the chart's objects, the permission is right
- [ ] 4.6 Confirm the read-only root filesystem holds: no restarts, no write errors in `kubectl logs`
- [ ] 4.7 Delete one pod and confirm the PDB allows it while keeping one available; check `kubectl get pdb` shows `ALLOWED DISRUPTIONS 1`
- [ ] 4.8 Re-run `kubectl apply -k deploy/` and confirm the replica count is not reset

## 5. Documentation

- [ ] 5.1 README: a "Run it in the cluster" section — `kubectl apply -k deploy/`, the `port-forward`, that there is deliberately no Ingress and why, and that the pod reads every Secret in the cluster
- [ ] 5.2 Record the commands in the project command reference: `kubectl kustomize`, the server dry-run, the port-forward, `kubectl get pdb`, `kubectl top pod` for revising the resource numbers
- [ ] 5.3 Note in `CLAUDE.md` that the sentinel context name is mapped in `Service.clientFor` and why it cannot be passed through to `LoadClient`
- [ ] 5.4 Add the increment to the README roadmap