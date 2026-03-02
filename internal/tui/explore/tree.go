package explore

// TreeNode represents a node in the expandable tree view.
type TreeNode struct {
	Item     FestivalItem
	Depth    int // 0=status, 1=festival, 2=phase, 3=sequence, 4=task
	Expanded bool
	Loading  bool // true while async child load in progress
	Loaded   bool // true once children fetched (even if empty)
	Children []*TreeNode
	Parent   *TreeNode
}

// IsLeaf returns true for task nodes that cannot have children.
func (n *TreeNode) IsLeaf() bool {
	return n.Item.Type == ItemTask
}

// NodeID returns a stable identifier for this node (its filesystem path).
func (n *TreeNode) NodeID() string {
	return n.Item.Path
}

// flattenTree performs a depth-first walk emitting only visible nodes
// (i.e., nodes whose ancestors are all expanded).
func flattenTree(roots []*TreeNode) []*TreeNode {
	var result []*TreeNode
	for _, root := range roots {
		flattenNode(root, &result)
	}
	return result
}

func flattenNode(node *TreeNode, result *[]*TreeNode) {
	*result = append(*result, node)
	if !node.Expanded {
		return
	}
	for _, child := range node.Children {
		flattenNode(child, result)
	}
}

// findNode searches the tree by path using DFS.
func findNode(roots []*TreeNode, path string) *TreeNode {
	for _, root := range roots {
		if found := findNodeDFS(root, path); found != nil {
			return found
		}
	}
	return nil
}

func findNodeDFS(node *TreeNode, path string) *TreeNode {
	if node.NodeID() == path {
		return node
	}
	for _, child := range node.Children {
		if found := findNodeDFS(child, path); found != nil {
			return found
		}
	}
	return nil
}

// itemsToTreeNodes converts loaded FestivalItems into TreeNode children.
func itemsToTreeNodes(items []FestivalItem, depth int, parent *TreeNode) []*TreeNode {
	nodes := make([]*TreeNode, 0, len(items))
	for _, item := range items {
		node := &TreeNode{
			Item:   item,
			Depth:  depth,
			Parent: parent,
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// breadcrumbsFromNode builds the breadcrumb chain by walking up the parent pointers.
func breadcrumbsFromNode(node *TreeNode) []string {
	if node == nil {
		return nil
	}
	var parts []string
	for n := node; n != nil; n = n.Parent {
		parts = append(parts, n.Item.Name)
	}
	// Reverse to get root-first order
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return parts
}
