package engine

import (
	"code.google.com/p/mx3/cuda"
)

// Stores values defined on the faces in-between cells.
// E.g.: the exchange interaction between cells.
// Automatically initialized to all ones.
type StaggeredMask struct {
	buffered
}

// Set the value of all cell faces with their normal along direction. E.g.:
// 	SetAll(X, 1) // sets all faces in YZ plane to value 1.
func (m *StaggeredMask) SetAll(direction int, value float64) {
	m.init()
	cuda.Memset(m.Slice.Comp(swapIndex(direction, 3)), float32(value))
}

// Sets the value at the "lower/left" face of cell(ix, iy, iz). E.g.:
// 	SetSide1(X, i, j, k, value) // sets value on the left side of cell i,j,k.
// 	SetSide1(Y, i, j, k, value) // sets value below (along Y) of cell i,j,k.
// 	SetSide1(Z, i, j, k, value) // sets value below (along Z) of cell i,j,k.
func (m *StaggeredMask) SetSide1(direction int, ix, iy, iz int, value float64) {
	m.init()
	cuda.SetCell(m.Slice, swapIndex(direction, 3), iz, iy, ix, float32(value))
}

// Sets the value at the "upper/right" face of cell(ix, iy, iz). E.g.:
// 	SetSide2(X, i, j, k, value) // sets value on the right side of cell i,j,k.
// 	SetSide2(Y, i, j, k, value) // sets value above (along Y) of cell i,j,k.
// 	SetSide2(Z, i, j, k, value) // sets value above (along Z) of cell i,j,k.
func (m *StaggeredMask) SetSide2(direction int, ix, iy, iz int, value float64) {
	m.init()
	direction = swapIndex(direction, 3)
	i := [3]int{iz, iy, ix}
	i[direction]++
	size := m.Mesh().Size()
	if i[direction] == size[direction] {
		i[direction] = 0 // wrap around boundary
	}
	cuda.SetCell(m.Slice, direction, i[0], i[1], i[2], float32(value))
}

func (m *StaggeredMask) init() {
	if m.Slice == nil {
		m.Slice = cuda.NewSlice(3, &mesh) // could alloc only needed components...
		cuda.Memset(m.Slice, 1, 1, 1)     // default value: all ones.
		onFree(func() { m.Slice.Free(); m.Slice = nil })
	}
}