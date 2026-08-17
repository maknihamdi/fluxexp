## 1. Cluster contract

- [x] 1.1 Add `List(ctx, apiVersion, kind, namespace) ([]unstructured, error)` to `resolver.K8sGetter`
- [x] 1.2 Update resolver test fakes to implement `List`; simplify the UI `cluster` interface to embed `K8sGetter` (List now comes from it)

## 2. Owner-descent helper

- [x] 2.1 `ownedChildren(ctx, rc, parentUID, ns, childAPIVersion, childKind, keep func(*unstructured) bool) ([]engine.Ref, error)`: list childKind in ns, keep those with an ownerReference UID == parentUID (and passing `keep`)
- [x] 2.2 Unit test: only UID-owned objects are returned

## 3. Workload descent

- [x] 3.1 Deployment: children = owned ReplicaSets with `status.replicas` > 0; append `· N old revisions` to detail when old (0-replica) revisions are omitted
- [x] 3.2 ReplicaSet / StatefulSet / DaemonSet / Job: children = owned Pods
- [x] 3.3 Pod: no children (leaf)
- [x] 3.4 Keep health computation unchanged; only children change

## 4. Tests

- [x] 4.1 Owned-only: a ReplicaSet owned by another Deployment is excluded (satisfies owner-descent scenario)
- [x] 4.2 Deployment: active ReplicaSet returned, 0-replica ones omitted, detail notes omitted count
- [x] 4.3 ReplicaSet → owned Pods; Pod → leaf
- [x] 4.4 Health still correct alongside children

## 5. Verification

- [x] 5.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --strict`
- [x] 5.2 Manual smoke: `traverse` reaches Deployment → ReplicaSet → Pods on a real workload; same via `ui` single-hop expansion
- [x] 5.3 Update `README.md` roadmap
