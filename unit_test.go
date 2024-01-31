package calcu

import (
	"fmt"
	"testing"
)

func TestDimensionPermutations(t *testing.T) {
	// we only need permutation with length of 2
	// assuming the first is num, and the second is den
	dims := []Dimension{DimEnergy, DimMass}
	var arr []Dimension
	var permutations [][]Dimension
	permute(arr, &permutations, dims, 2)
	want, got := 4, len(permutations)
	if want != len(permutations) {
		t.Errorf("permute: got %q, want %q", got, want)
	}
}

func TestStaticUnitManager(t *testing.T) {
	um := newStaticUintManager()
	sd := um.(*staticum)
	fmt.Println(len(sd.names))
}
