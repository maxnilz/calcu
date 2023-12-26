package calcu

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"sort"

	"github.com/shopspring/decimal"
)

type UnitManager interface {
	// Peek check if start of s is a unit
	// if it's a unit, return the unit len
	Peek(s string) (int, bool)
	// IsUnit check if the given s is a unit
	IsUnit(s string) bool
	GetByName(name string) (Unit, bool)
	ListMetaUnitsByDims(dim ...Dimension) ([]*MetaUnit, error)
}

type Dimension int

const (
	DimInvalid Dimension = iota
	DimEnergy
	DimMass
	DimVolume
	DimTime
	DimLength
)

func DimensionFromString(s string) Dimension {
	d := DimInvalid
	switch s {
	case "Energy":
		d = DimEnergy
	case "Mass":
		d = DimMass
	case "Volume":
		d = DimVolume
	case "Time":
		d = DimTime
	case "Length":
		d = DimLength
	}
	return d
}

type Unit interface {
	Name() string
	Label() string
	Dimension() Dimension
	IsMeta() bool
	SiName() string
	SiFactors() (decimal.Decimal, decimal.Decimal)
}

type MetaUnit struct {
	name      string
	label     string
	dimension Dimension
	si        string
	siFactor  decimal.Decimal
	siOffset  decimal.Decimal
}

func (u *MetaUnit) Name() string {
	return u.name
}

func (u *MetaUnit) Label() string {
	return u.label
}

func (u *MetaUnit) Dimension() Dimension {
	return u.dimension
}

func (u *MetaUnit) IsMeta() bool {
	return true
}

func (u *MetaUnit) SiName() string {
	return u.si
}

func (u *MetaUnit) SiFactors() (decimal.Decimal, decimal.Decimal) {
	return u.siFactor, u.siOffset
}

// CompoundUnit represent as Numerator/Denominator
// Numerator and Denominator should have different
// dimensions, for example: j/kg, Gg/Tj
type CompoundUnit struct {
	Numerator   *MetaUnit
	Denominator *MetaUnit
	SiFactor    decimal.Decimal
}

func newCompoundUnit(num, den *MetaUnit) *CompoundUnit {
	// num as Numerator, den as Denominator
	// e.g: energy unit: Tj to J(SI) is 1,000,000,000,000
	// mass unit: Gg to kg(SI) is 1,000,000, then SI of
	// Tj/Gg is 1,000,000,000,000/1,000,000 i.e, 1,000,000
	numFactor := num.siFactor
	denFactor := den.siFactor
	return &CompoundUnit{
		Numerator:   num,
		Denominator: den,
		SiFactor:    numFactor.Div(denFactor),
	}
}

func (u *CompoundUnit) Name() string {
	return fmt.Sprintf("%s/%s", u.Numerator.name, u.Denominator.name)
}

func (u *CompoundUnit) Label() string {
	return u.Name()
}

func (u *CompoundUnit) Dimension() Dimension {
	return DimInvalid
}

func (u *CompoundUnit) IsMeta() bool {
	return false
}

func (u *CompoundUnit) SiName() string {
	return fmt.Sprintf("%s/%s", u.Numerator.si, u.Denominator.si)
}

func (u *CompoundUnit) SiFactors() (decimal.Decimal, decimal.Decimal) {
	return u.SiFactor, decimal.Zero
}

func MaybeAmbiguousUnitName(name string) (string, bool) {
	// if the first char of unit
	// is a digit, we use brackets
	// to remove ambiguity.
	c := name[0]
	if c >= 49 && c <= 57 {
		return "(" + name + ")", true
	}
	return name, false
}

//go:embed unit.csv
var unitAsset embed.FS

type staticum struct {
	m map[string]Unit

	dimMUnits map[Dimension][]*MetaUnit

	ulens []int
	names []string
}

func newStaticUintManager() UnitManager {
	m := make(map[string]Unit)
	dimMUnits := make(map[Dimension][]*MetaUnit)
	var names []string

	f, _ := unitAsset.Open("unit.csv")
	rd := csv.NewReader(f)
	rowid := 0
	for ; ; rowid++ {
		record, err := rd.Read()
		if err == io.EOF {
			break
		}
		if rowid == 0 {
			continue // skip header row
		}
		dimension := DimensionFromString(record[2])
		siFactor, _ := decimal.NewFromString(record[4])
		siOffset, _ := decimal.NewFromString(record[5])
		u := MetaUnit{
			name:      record[0],
			label:     record[1],
			dimension: dimension,
			si:        record[3],
			siFactor:  siFactor,
			siOffset:  siOffset,
		}
		m[u.name] = &u
		dimMUnits[u.dimension] = append(dimMUnits[u.dimension], &u)
		names = append(names, u.name)
		if s, ok := MaybeAmbiguousUnitName(u.name); ok {
			names = append(names, s)
		}
	}

	// permutations for dimensions to build compound units
	dims := []Dimension{DimEnergy, DimMass, DimVolume, DimTime, DimLength}
	var arr []Dimension
	var permutations [][]Dimension
	// we only need permutation with length of 2
	// assuming the first is num, and the second is den
	permute(arr, &permutations, dims, 2)
	// generate all possible compound units
	for _, p := range permutations {
		num, den := p[0], p[1]
		nums := dimMUnits[num]
		dens := dimMUnits[den]
		for i := 0; i < len(nums); i++ {
			for j := 0; j < len(dens); j++ {
				cu := newCompoundUnit(nums[i], dens[j])
				names = append(names, cu.Name())
				if s, ok := MaybeAmbiguousUnitName(cu.Name()); ok {
					names = append(names, s)
				}
				m[cu.Name()] = cu
			}
		}
	}
	// order names by length for the peek operation
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})
	ulens := make([]int, len(names))
	for i, name := range names {
		ulens[i] = len(name)
	}
	return &staticum{names: names, ulens: ulens, m: m, dimMUnits: dimMUnits}
}

func permute(arr []Dimension, ans *[][]Dimension, dims []Dimension, depth int) {
	if len(arr) == depth {
		dst := make([]Dimension, len(arr))
		copy(dst, arr)
		*ans = append(*ans, dst)
		return
	}
	m := make(map[Dimension]struct{})
	for _, a := range arr {
		m[a] = struct{}{}
	}
	for _, dim := range dims {
		if _, ok := m[dim]; !ok {
			arr = append(arr, dim)
			permute(arr, ans, dims, depth)
			arr = arr[:len(arr)-1]
		}
	}
}

func (su *staticum) dimension(s string) (Dimension, bool) {
	u, ok := su.m[s]
	if !ok {
		return DimInvalid, false
	}
	mu, ok := u.(*MetaUnit)
	if ok {
		return mu.dimension, true
	}
	return DimInvalid, false
}

func (su *staticum) Peek(s string) (int, bool) {
	for i, name := range su.names {
		n := su.ulens[i]
		if len(s) < n {
			continue
		}
		a, b := s[:n], ""
		if len(s) > n {
			b = s[n:]
		}
		// consider the following char to
		// avoid mistake, i.e., the unit
		// token should only appear before
		// a separator char or end of line.
		// not in front of a char. for example,
		// consider unit Meter(m), without
		// check the following char, we might
		// treat the `m` in word `me` as a unit
		// or the `m` in `m = 1` will be treated
		// as a unit as well.
		if a == name && startWithSeparator(b) {
			return n, true
		}
	}
	return 0, false
}

func (su *staticum) IsUnit(s string) bool {
	_, ok := su.m[s]
	return ok
}

func (su *staticum) GetByName(name string) (Unit, bool) {
	u, ok := su.m[name]
	return u, ok
}

func (su *staticum) ListMetaUnitsByDims(dims ...Dimension) ([]*MetaUnit, error) {
	var ans []*MetaUnit
	for _, dim := range dims {
		ans = append(ans, su.dimMUnits[dim]...)
	}
	sort.SliceStable(ans, func(i, j int) bool {
		return ans[i].label < ans[j].label
	})
	return ans, nil
}

// StdUm a builtin static unit manager
// it should be used as read only purpose
// otherwise, we might need consider make
// it thread safe. one can replace it with
// other implementation globally with an
// atomic store.
var StdUm = newStaticUintManager()
