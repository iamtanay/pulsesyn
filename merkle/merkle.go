package merkle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// Sentinel errors returned by merkle operations.
var (
	ErrNoLeaves        = errors.New("pulsesyn/merkle: leaves slice is empty")
	ErrInvalidLeafIndex = errors.New("pulsesyn/merkle: leaf index out of range")
	ErrNilProof        = errors.New("pulsesyn/merkle: proof is nil")
)

// Tree is an immutable Merkle tree built from a set of leaf data items.
// Layers are stored unpadded; odd-layer padding is handled internally during
// build and proof generation. See DECISIONS.md: 2026-05-31.
type Tree struct {
	layers    [][][]byte
	leafCount int
}

// ProofNode is one element of a Merkle proof path — a sibling hash and its
// position relative to the current node.
type ProofNode struct {
	// Hash is the sibling node's hash at this level of the tree.
	Hash []byte

	// IsLeft is true when the sibling is the left child and the current node
	// is the right child. The hasher must apply: hash(sibling || current).
	IsLeft bool
}

// Proof is a Merkle inclusion proof for a specific leaf. It proves that the
// leaf at LeafIndex is included in the tree with the root produced by Build.
type Proof struct {
	// LeafIndex is the 0-based index of the leaf this proof covers.
	LeafIndex int

	// Nodes is the sequence of sibling hashes from leaf to root (exclusive).
	Nodes []ProofNode
}

// Build constructs a Merkle tree from a slice of leaf data. Each element is
// domain-separated and hashed to form a leaf node. Returns ErrNoLeaves if
// the slice is empty.
// See DECISIONS.md: 2026-05-31 (domain separation rationale).
func Build(leaves [][]byte) (*Tree, error) {
	if len(leaves) == 0 {
		return nil, ErrNoLeaves
	}

	leafHashes := make([][]byte, len(leaves))
	for i, leaf := range leaves {
		leafHashes[i] = hashLeaf(leaf)
	}

	tree := &Tree{leafCount: len(leaves)}
	current := leafHashes

	for {
		layer := make([][]byte, len(current))
		copy(layer, current)
		tree.layers = append(tree.layers, layer)

		if len(current) == 1 {
			break
		}

		// Pad to even length for building the next layer without modifying
		// the stored layer.
		working := current
		if len(working)%2 != 0 {
			working = make([][]byte, len(current)+1)
			copy(working, current)
			working[len(current)] = current[len(current)-1]
		}

		next := make([][]byte, len(working)/2)
		for i := 0; i < len(working); i += 2 {
			next[i/2] = hashInternal(working[i], working[i+1])
		}
		current = next
	}

	return tree, nil
}

// Root returns the root hash of the tree. The root is the single element at
// the top layer.
func (t *Tree) Root() []byte {
	root := t.layers[len(t.layers)-1][0]
	cp := make([]byte, len(root))
	copy(cp, root)
	return cp
}

// LeafCount returns the number of original leaves in the tree.
func (t *Tree) LeafCount() int {
	return t.leafCount
}

// GenerateProof returns the inclusion proof for the leaf at leafIndex.
// Returns ErrInvalidLeafIndex if leafIndex is out of range.
func (t *Tree) GenerateProof(leafIndex int) (*Proof, error) {
	if leafIndex < 0 || leafIndex >= t.leafCount {
		return nil, fmt.Errorf("GenerateProof: %w: %d (tree has %d leaves)", ErrInvalidLeafIndex, leafIndex, t.leafCount)
	}

	nodes := make([]ProofNode, 0, len(t.layers)-1)
	idx := leafIndex

	for level := 0; level < len(t.layers)-1; level++ {
		layer := t.layers[level]
		var sibling []byte
		var isLeft bool

		if idx%2 == 0 {
			// Current is the left child; sibling is the right.
			siblingIdx := idx + 1
			if siblingIdx < len(layer) {
				sibling = layer[siblingIdx]
			} else {
				// Odd layer — current was duplicated; sibling is itself.
				sibling = layer[idx]
			}
			isLeft = false
		} else {
			// Current is the right child; sibling is the left.
			sibling = layer[idx-1]
			isLeft = true
		}

		hashCopy := make([]byte, len(sibling))
		copy(hashCopy, sibling)
		nodes = append(nodes, ProofNode{Hash: hashCopy, IsLeft: isLeft})
		idx /= 2
	}

	return &Proof{LeafIndex: leafIndex, Nodes: nodes}, nil
}

// VerifyProof verifies that leafData is included in the tree identified by
// root, using the provided proof. Returns false if the proof is nil, if the
// leaf hash does not match, or if any step of the proof does not match root.
func VerifyProof(root []byte, leafData []byte, proof *Proof) bool {
	if proof == nil {
		return false
	}

	current := hashLeaf(leafData)

	for _, node := range proof.Nodes {
		if node.IsLeft {
			current = hashInternal(node.Hash, current)
		} else {
			current = hashInternal(current, node.Hash)
		}
	}

	return bytes.Equal(current, root)
}

// HashVote produces a deterministic leaf hash for a single validator vote,
// suitable for use as input to Build. The hash commits to all fields that
// affect the vote's weight in consensus.
func HashVote(validatorID, verdict string, confidence, domainReputation, biasCoefficient float64) []byte {
	h := sha256.New()
	h.Write([]byte(validatorID))
	h.Write([]byte("|"))
	h.Write([]byte(verdict))
	h.Write([]byte("|"))
	_ = binary.Write(h, binary.BigEndian, confidence)
	_ = binary.Write(h, binary.BigEndian, domainReputation)
	_ = binary.Write(h, binary.BigEndian, biasCoefficient)
	return h.Sum(nil)
}

// hashLeaf hashes a leaf data item with the domain separation prefix 0x00.
// See DECISIONS.md: 2026-05-31.
func hashLeaf(data []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(data)
	return h.Sum(nil)
}

// hashInternal hashes two child node hashes with the domain separation prefix
// 0x01.
func hashInternal(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}
