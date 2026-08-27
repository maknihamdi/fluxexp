package engine

// Traverse walks the resource graph starting from root, using resolve to obtain
// each node's health and children. It is domain-agnostic and performs no backend
// I/O itself: every retrieval happens inside resolve.
//
// Properties:
//   - read-only: the engine never mutates anything;
//   - cycle-safe: each reference (by Ref.Key) is resolved at most once; a
//     repeated reference is attached as an already-visited leaf pointer;
//   - partial-failure-tolerant: a reference whose resolve returns an error
//     becomes an Error node and traversal of siblings/subtrees continues.
//
// Traversal is breadth-first, so output order is stable and bounded regardless
// of graph depth.
func Traverse(root Ref, resolve ResolveFunc) *Node {
	visited := map[string]*Node{}

	rootNode, rootChildren := resolveNode(root, resolve)
	visited[root.Key()] = rootNode
	resolveDependencies(rootNode, resolve)

	type frame struct {
		parent *Node
		refs   []Ref
	}
	queue := []frame{{parent: rootNode, refs: rootChildren}}

	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]

		for _, ref := range f.refs {
			if prior, seen := visited[ref.Key()]; seen {
				// Already expanded elsewhere: attach a leaf pointer carrying the
				// prior health, but do not re-expand (cycle/diamond safety).
				f.parent.Children = append(f.parent.Children, &Node{
					Ref:        ref,
					Health:     prior.Health,
					Detail:     prior.Detail,
					Expandable: prior.Expandable,
					Visited:    true,
				})
				continue
			}

			childNode, grandChildren := resolveNode(ref, resolve)
			visited[ref.Key()] = childNode
			resolveDependencies(childNode, resolve)
			f.parent.Children = append(f.parent.Children, childNode)

			if len(grandChildren) > 0 {
				queue = append(queue, frame{parent: childNode, refs: grandChildren})
			}
		}
	}

	return rootNode
}

// resolveDependencies resolves each of node's declared dependencies one level
// deep and attaches them. Their own children are deliberately discarded: a
// dependency answers "what does this need?", not "what is under it", and
// descending would splice whole subtrees into the tree.
//
// Dependencies are also kept out of the caller's visited map on purpose. Marking
// one visited would cause a later, genuine child occurrence of the same
// reference to be attached as an already-visited pointer and lose its subtree.
func resolveDependencies(node *Node, resolve ResolveFunc) {
	if node == nil || len(node.deps) == 0 {
		return
	}
	for _, ref := range node.deps {
		depNode, _ := resolveNode(ref, resolve)
		depNode.Dependency = true
		node.Dependencies = append(node.Dependencies, depNode)
	}
	node.deps = nil
}

// resolveNode resolves a single reference into a Node plus the child references
// to enqueue. A resolve error yields an Error node with no children.
func resolveNode(ref Ref, resolve ResolveFunc) (*Node, []Ref) {
	res, err := resolve(ref)
	if err != nil {
		return &Node{Ref: ref, Health: Error, Err: err.Error()}, nil
	}
	return &Node{
		Ref:        ref,
		Health:     res.Health,
		Detail:     res.Detail,
		Freshness:  res.Freshness,
		Fields:     res.Fields,
		Expandable: res.Expandable,
		deps:       res.Dependencies,
	}, res.Children
}
