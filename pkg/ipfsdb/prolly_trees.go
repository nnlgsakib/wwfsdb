package ipfsdb

import (
	"bytes"
	"fmt"
	"sort"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"google.golang.org/protobuf/proto"
)

const (
	// ProllyTreeOrder defines the branching factor of the tree.
	ProllyTreeOrder = 8 // Using a smaller order for easier testing and demonstration
	// MaxKeys is the maximum number of keys in a node.
	MaxKeys = ProllyTreeOrder - 1
	// MinKeys is the minimum number of keys in a node (except the root).
	MinKeys = (ProllyTreeOrder / 2) - 1
)

// ProllyTree represents the entire tree structure.
type ProllyTree struct {
	RootCID string
	sh      *shell.Shell
}

// --- Core IPFS/Serialization Helpers ---

// loadNode deserializes a ProllyNode from IPFS.
func (t *ProllyTree) loadNode(cid string) (*pb.ProllyNode, error) {
	if cid == "" {
		return nil, fmt.Errorf("cannot load empty cid")
	}
	data, err := t.sh.Cat(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to cat node %s: %w", cid, err)
	}
	defer data.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	var node pb.ProllyNode
	if err := proto.Unmarshal(buf.Bytes(), &node); err != nil {
		return nil, fmt.Errorf("failed to decode node %s: %w", cid, err)
	}
	return &node, nil
}

// saveNode serializes a ProllyNode and saves it to IPFS, returning its CID.
func (t *ProllyTree) saveNode(node *pb.ProllyNode) (string, error) {
	data, err := proto.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node: %w", err)
	}
	return t.sh.Add(bytes.NewReader(data), shell.CidVersion(1))
}

// --- Tree Initialization ---

// NewProllyTree creates a new, empty tree with a single empty leaf node.
func NewProllyTree(sh *shell.Shell) (*ProllyTree, error) {
	emptyNode := &pb.ProllyNode{
		IsLeaf: true,
	}
	cid, err := (&ProllyTree{sh: sh}).saveNode(emptyNode)
	if err != nil {
		return nil, fmt.Errorf("failed to save initial empty node: %w", err)
	}
	return &ProllyTree{
		RootCID: cid,
		sh:      sh,
	}, nil
}

// LoadProllyTree loads an existing tree from a root CID.
func LoadProllyTree(sh *shell.Shell, rootCID string) *ProllyTree {
	return &ProllyTree{
		RootCID: rootCID,
		sh:      sh,
	}
}

// --- Public API: Get, Put, Delete ---

// Get searches for a key and returns the corresponding values.
func (t *ProllyTree) Get(key string) ([]string, error) {
	if t.RootCID == "" {
		return nil, nil // Empty tree
	}
	return t.recursiveGet(t.RootCID, key)
}

// Put inserts a key-value pair. If the key exists, it appends the value.
// It returns a new ProllyTree with the updated root CID.
func (t *ProllyTree) Put(key, value string) (*ProllyTree, error) {
	newRootCID, promotedKey, promotedChildCID, err := t.recursivePut(t.RootCID, key, value)
	if err != nil {
		return nil, err
	}

	// If recursivePut promoted a key, the root was split. Create a new root.
	if promotedKey != "" {
		newRoot := &pb.ProllyNode{
			Keys:     []string{promotedKey},
			Children: []string{newRootCID, promotedChildCID},
			IsLeaf:   false,
		}
		newRootCID, err = t.saveNode(newRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to create new root: %w", err)
		}
	}

	return &ProllyTree{RootCID: newRootCID, sh: t.sh}, nil
}

// Delete removes a key and its values from the tree.
func (t *ProllyTree) Delete(key string) (*ProllyTree, error) {
	newRootCID, err := t.recursiveDelete(t.RootCID, key)
	if err != nil {
		return nil, err
	}

	// If the root node is now empty (after a merge), the new root is its only child.
	root, err := t.loadNode(newRootCID)
	if err != nil {
		return nil, err
	}
	if !root.IsLeaf && len(root.Keys) == 0 && len(root.Children) > 0 {
		newRootCID = root.Children[0]
	}

	return &ProllyTree{RootCID: newRootCID, sh: t.sh}, nil
}

// --- Recursive Implementations ---

func (t *ProllyTree) recursiveGet(nodeCID, key string) ([]string, error) {
	node, err := t.loadNode(nodeCID)
	if err != nil {
		return nil, err
	}

	i := sort.SearchStrings(node.Keys, key)

	if node.IsLeaf {
		if i < len(node.Keys) && node.Keys[i] == key {
			return node.Values[i].Values, nil
		}
		return nil, nil // Not found
	} else {
		if i < len(node.Keys) && node.Keys[i] == key {
			i++ // Key is in the right subtree of the key
		}
		return t.recursiveGet(node.Children[i], key)
	}
}

func (t *ProllyTree) recursivePut(nodeCID, key, value string) (string, string, string, error) {
	node, err := t.loadNode(nodeCID)
	if err != nil {
		return "", "", "", err
	}

	i := sort.SearchStrings(node.Keys, key)

	if node.IsLeaf {
		if i < len(node.Keys) && node.Keys[i] == key {
			node.Values[i].Values = append(node.Values[i].Values, value)
		} else {
			node.Keys = append(node.Keys[:i], append([]string{key}, node.Keys[i:]...)...)
			newValue := &pb.ProllyNodeValues{Values: []string{value}}
			node.Values = append(node.Values[:i], append([]*pb.ProllyNodeValues{newValue}, node.Values[i:]...)...)
		}

		if len(node.Keys) > MaxKeys {
			mid := len(node.Keys) / 2
			promotedKey := node.Keys[mid]

			newNode := &pb.ProllyNode{
				Keys:   node.Keys[mid+1:],
				Values: node.Values[mid+1:],
				IsLeaf: true,
			}
			node.Keys = node.Keys[:mid]
			node.Values = node.Values[:mid]

			newCID, err := t.saveNode(newNode)
			if err != nil {
				return "", "", "", err
			}
			thisCID, err := t.saveNode(node)
			if err != nil {
				return "", "", "", err
			}
			return thisCID, promotedKey, newCID, nil
		}

		newCID, err := t.saveNode(node)
		return newCID, "", "", err
	}

	// Internal node logic
	childCID, promotedKey, promotedChildCID, err := t.recursivePut(node.Children[i], key, value)
	if err != nil {
		return "", "", "", err
	}
	node.Children[i] = childCID

	if promotedKey != "" {
		node.Keys = append(node.Keys[:i], append([]string{promotedKey}, node.Keys[i:]...)...)
		node.Children = append(node.Children[:i+1], append([]string{promotedChildCID}, node.Children[i+1:]...)...)

		if len(node.Keys) > MaxKeys {
			mid := len(node.Keys) / 2
			promotedKey := node.Keys[mid]
			newNode := &pb.ProllyNode{
				Keys:     node.Keys[mid+1:],
				Children: node.Children[mid+1:],
				IsLeaf:   false,
			}
			node.Keys = node.Keys[:mid]
			node.Children = node.Children[:mid+1]

			newCID, err := t.saveNode(newNode)
			if err != nil {
				return "", "", "", err
			}
			thisCID, err := t.saveNode(node)
			if err != nil {
				return "", "", "", err
			}
			return thisCID, promotedKey, newCID, nil
		}
	}

	newCID, err := t.saveNode(node)
	return newCID, "", "", err
}

func (t *ProllyTree) recursiveDelete(nodeCID, key string) (string, error) {
	node, err := t.loadNode(nodeCID)
	if err != nil {
		return "", err
	}

	i := sort.SearchStrings(node.Keys, key)

	if node.IsLeaf {
		if i < len(node.Keys) && node.Keys[i] == key {
			node.Keys = append(node.Keys[:i], node.Keys[i+1:]...)
			node.Values = append(node.Values[:i], node.Values[i+1:]...)
		} else {
			return nodeCID, nil // Key not found, no change
		}
	} else { // Internal node
		newChildCID, err := t.recursiveDelete(node.Children[i], key)
		if err != nil {
			return "", err
		}

		if newChildCID == node.Children[i] {
			return nodeCID, nil // No change in subtree
		}

		node.Children[i] = newChildCID
		child, err := t.loadNode(newChildCID)
		if err != nil {
			return "", err
		}

		if len(child.Keys) < MinKeys {
			node, err = t.handleUnderflow(node, i)
			if err != nil {
				return "", err
			}
		}
	}

	return t.saveNode(node)
}

func (t *ProllyTree) handleUnderflow(parent *pb.ProllyNode, childIdx int) (*pb.ProllyNode, error) {
	// Try to borrow from the left sibling
	if childIdx > 0 {
		leftSibling, err := t.loadNode(parent.Children[childIdx-1])
		if err != nil {
			return nil, err
		}
		if len(leftSibling.Keys) > MinKeys {
			return t.borrowFromLeft(parent, childIdx, leftSibling)
		}
	}

	// Try to borrow from the right sibling
	if childIdx < len(parent.Keys) {
		rightSibling, err := t.loadNode(parent.Children[childIdx+1])
		if err != nil {
			return nil, err
		}
		if len(rightSibling.Keys) > MinKeys {
			return t.borrowFromRight(parent, childIdx, rightSibling)
		}
	}

	// Must merge
	if childIdx > 0 {
		return t.mergeWithLeft(parent, childIdx)
	} else {
		return t.mergeWithRight(parent, childIdx)
	}
}

func (t *ProllyTree) borrowFromLeft(parent *pb.ProllyNode, childIdx int, leftSibling *pb.ProllyNode) (*pb.ProllyNode, error) {
	child, err := t.loadNode(parent.Children[childIdx])
	if err != nil {
		return nil, err
	}

	// Move separator key from parent to child
	child.Keys = append([]string{parent.Keys[childIdx-1]}, child.Keys...)
	parent.Keys[childIdx-1] = leftSibling.Keys[len(leftSibling.Keys)-1]

	if child.IsLeaf {
		child.Values = append([]*pb.ProllyNodeValues{leftSibling.Values[len(leftSibling.Values)-1]}, child.Values...)
		leftSibling.Values = leftSibling.Values[:len(leftSibling.Values)-1]
	} else {
		child.Children = append([]string{leftSibling.Children[len(leftSibling.Children)-1]}, child.Children...)
		leftSibling.Children = leftSibling.Children[:len(leftSibling.Children)-1]
	}
	leftSibling.Keys = leftSibling.Keys[:len(leftSibling.Keys)-1]

	// Save modified nodes
	newChildCID, err := t.saveNode(child)
	if err != nil {
		return nil, err
	}
	newLeftSiblingCID, err := t.saveNode(leftSibling)
	if err != nil {
		return nil, err
	}

	parent.Children[childIdx] = newChildCID
	parent.Children[childIdx-1] = newLeftSiblingCID
	return parent, nil
}

func (t *ProllyTree) borrowFromRight(parent *pb.ProllyNode, childIdx int, rightSibling *pb.ProllyNode) (*pb.ProllyNode, error) {
	child, err := t.loadNode(parent.Children[childIdx])
	if err != nil {
		return nil, err
	}

	// Move separator key from parent to child
	child.Keys = append(child.Keys, parent.Keys[childIdx])
	parent.Keys[childIdx] = rightSibling.Keys[0]

	if child.IsLeaf {
		child.Values = append(child.Values, rightSibling.Values[0])
		rightSibling.Values = rightSibling.Values[1:]
	} else {
		child.Children = append(child.Children, rightSibling.Children[0])
		rightSibling.Children = rightSibling.Children[1:]
	}
	rightSibling.Keys = rightSibling.Keys[1:]

	// Save modified nodes
	newChildCID, err := t.saveNode(child)
	if err != nil {
		return nil, err
	}
	newRightSiblingCID, err := t.saveNode(rightSibling)
	if err != nil {
		return nil, err
	}

	parent.Children[childIdx] = newChildCID
	parent.Children[childIdx+1] = newRightSiblingCID
	return parent, nil
}

func (t *ProllyTree) mergeWithLeft(parent *pb.ProllyNode, childIdx int) (*pb.ProllyNode, error) {
	child, err := t.loadNode(parent.Children[childIdx])
	if err != nil {
		return nil, err
	}
	leftSibling, err := t.loadNode(parent.Children[childIdx-1])
	if err != nil {
		return nil, err
	}

	// Pull separator key from parent into left sibling
	leftSibling.Keys = append(leftSibling.Keys, parent.Keys[childIdx-1])
	// Merge child into left sibling
	leftSibling.Keys = append(leftSibling.Keys, child.Keys...)

	if leftSibling.IsLeaf {
		leftSibling.Values = append(leftSibling.Values, child.Values...)
	} else {
		leftSibling.Children = append(leftSibling.Children, child.Children...)
	}

	// Remove key and child from parent
	parent.Keys = append(parent.Keys[:childIdx-1], parent.Keys[childIdx:]...)
	parent.Children = append(parent.Children[:childIdx], parent.Children[childIdx+1:]...)

	// Save new merged node
	newLeftSiblingCID, err := t.saveNode(leftSibling)
	if err != nil {
		return nil, err
	}
	parent.Children[childIdx-1] = newLeftSiblingCID

	return parent, nil
}

func (t *ProllyTree) mergeWithRight(parent *pb.ProllyNode, childIdx int) (*pb.ProllyNode, error) {
	child, err := t.loadNode(parent.Children[childIdx])
	if err != nil {
		return nil, err
	}
	rightSibling, err := t.loadNode(parent.Children[childIdx+1])
	if err != nil {
		return nil, err
	}

	// Pull separator key from parent into child
	child.Keys = append(child.Keys, parent.Keys[childIdx])
	// Merge right sibling into child
	child.Keys = append(child.Keys, rightSibling.Keys...)

	if child.IsLeaf {
		child.Values = append(child.Values, rightSibling.Values...)
	} else {
		child.Children = append(child.Children, rightSibling.Children...)
	}

	// Remove key and child from parent
	parent.Keys = append(parent.Keys[:childIdx], parent.Keys[childIdx+1:]...)
	parent.Children = append(parent.Children[:childIdx+1], parent.Children[childIdx+2:]...)

	// Save new merged node
	newChildCID, err := t.saveNode(child)
	if err != nil {
		return nil, err
	}
	parent.Children[childIdx] = newChildCID

	return parent, nil
}