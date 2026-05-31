package merkle

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leaves returns a slice of n distinct byte slices for testing.
func leaves(n int) [][]byte {
	result := make([][]byte, n)
	for i := range result {
		result[i] = []byte{byte(i + 1)}
	}
	return result
}

// --- Build ---

func TestBuild_EmptyLeaves(t *testing.T) {
	_, err := Build(nil)
	require.ErrorIs(t, err, ErrNoLeaves)

	_, err = Build([][]byte{})
	require.ErrorIs(t, err, ErrNoLeaves)
}

func TestBuild_SingleLeaf(t *testing.T) {
	tree, err := Build(leaves(1))
	require.NoError(t, err)
	assert.Equal(t, 1, tree.LeafCount())
	assert.Len(t, tree.Root(), 32)
}

func TestBuild_TwoLeaves(t *testing.T) {
	tree, err := Build(leaves(2))
	require.NoError(t, err)
	assert.Equal(t, 2, tree.LeafCount())
	assert.Len(t, tree.Root(), 32)
}

func TestBuild_OddLeaves(t *testing.T) {
	// Odd count triggers last-leaf duplication. Must still produce a valid tree.
	tree, err := Build(leaves(3))
	require.NoError(t, err)
	assert.Equal(t, 3, tree.LeafCount())
	assert.Len(t, tree.Root(), 32)
}

func TestBuild_EvenLeaves(t *testing.T) {
	tree, err := Build(leaves(4))
	require.NoError(t, err)
	assert.Equal(t, 4, tree.LeafCount())
}

func TestRoot_Deterministic(t *testing.T) {
	ls := leaves(5)
	t1, err := Build(ls)
	require.NoError(t, err)
	t2, err := Build(ls)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(t1.Root(), t2.Root()))
}

func TestRoot_DifferentLeavesProduceDifferentRoots(t *testing.T) {
	ls1 := [][]byte{[]byte("a"), []byte("b")}
	ls2 := [][]byte{[]byte("a"), []byte("c")}
	t1, _ := Build(ls1)
	t2, _ := Build(ls2)
	assert.False(t, bytes.Equal(t1.Root(), t2.Root()))
}

// --- GenerateProof ---

func TestGenerateProof_InvalidIndex(t *testing.T) {
	tree, err := Build(leaves(3))
	require.NoError(t, err)

	_, err = tree.GenerateProof(-1)
	require.ErrorIs(t, err, ErrInvalidLeafIndex)

	_, err = tree.GenerateProof(3)
	require.ErrorIs(t, err, ErrInvalidLeafIndex)
}

func TestGenerateProof_SingleLeaf(t *testing.T) {
	ls := [][]byte{[]byte("only")}
	tree, err := Build(ls)
	require.NoError(t, err)

	proof, err := tree.GenerateProof(0)
	require.NoError(t, err)
	assert.Empty(t, proof.Nodes)
	assert.Equal(t, 0, proof.LeafIndex)
}

// --- VerifyProof ---

func TestVerifyProof_NilProof(t *testing.T) {
	tree, _ := Build(leaves(2))
	assert.False(t, VerifyProof(tree.Root(), []byte{1}, nil))
}

func TestVerifyProof_SingleLeaf(t *testing.T) {
	leaf := []byte("solo")
	tree, err := Build([][]byte{leaf})
	require.NoError(t, err)
	proof, err := tree.GenerateProof(0)
	require.NoError(t, err)
	assert.True(t, VerifyProof(tree.Root(), leaf, proof))
}

func TestVerifyProof_TamperedLeaf(t *testing.T) {
	ls := leaves(4)
	tree, err := Build(ls)
	require.NoError(t, err)
	proof, err := tree.GenerateProof(2)
	require.NoError(t, err)
	// Present a different leaf with the proof for leaf 2.
	assert.False(t, VerifyProof(tree.Root(), ls[3], proof))
}

func TestVerifyProof_TamperedProofNode(t *testing.T) {
	ls := leaves(4)
	tree, err := Build(ls)
	require.NoError(t, err)
	proof, err := tree.GenerateProof(1)
	require.NoError(t, err)
	// Corrupt the first sibling hash.
	proof.Nodes[0].Hash[0] ^= 0xFF
	assert.False(t, VerifyProof(tree.Root(), ls[1], proof))
}

func TestVerifyProof_WrongRoot(t *testing.T) {
	ls := leaves(4)
	tree, err := Build(ls)
	require.NoError(t, err)
	proof, err := tree.GenerateProof(0)
	require.NoError(t, err)
	wrongRoot := make([]byte, 32)
	assert.False(t, VerifyProof(wrongRoot, ls[0], proof))
}

// --- Round-trip: build, prove every leaf, verify ---

func TestRoundTrip_AllLeaves(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 7, 8, 13} {
		ls := leaves(n)
		tree, err := Build(ls)
		require.NoError(t, err, "Build n=%d", n)
		root := tree.Root()

		for i, leaf := range ls {
			proof, err := tree.GenerateProof(i)
			require.NoError(t, err, "GenerateProof n=%d i=%d", n, i)
			assert.True(t, VerifyProof(root, leaf, proof), "VerifyProof n=%d i=%d", n, i)
		}
	}
}

// --- HashVote ---

func TestHashVote_Deterministic(t *testing.T) {
	h1 := HashVote("validator-1", "SUPPORTED", 0.85, 0.75, 0.05)
	h2 := HashVote("validator-1", "SUPPORTED", 0.85, 0.75, 0.05)
	assert.True(t, bytes.Equal(h1, h2))
}

func TestHashVote_DistinctInputsDistinctHashes(t *testing.T) {
	h1 := HashVote("validator-1", "SUPPORTED", 0.85, 0.75, 0.05)
	h2 := HashVote("validator-1", "UNSUPPORTED", 0.85, 0.75, 0.05)
	assert.False(t, bytes.Equal(h1, h2))
}

func TestHashVote_UsableAsLeaf(t *testing.T) {
	votes := [][]byte{
		HashVote("v-001", "SUPPORTED", 0.85, 0.80, 0.02),
		HashVote("v-002", "SUPPORTED", 0.90, 0.75, 0.01),
		HashVote("v-003", "UNSUPPORTED", 0.60, 0.70, 0.15),
	}
	tree, err := Build(votes)
	require.NoError(t, err)
	for i, vote := range votes {
		proof, err := tree.GenerateProof(i)
		require.NoError(t, err)
		assert.True(t, VerifyProof(tree.Root(), vote, proof))
	}
}
