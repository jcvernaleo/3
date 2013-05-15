package engine

import (
	"code.google.com/p/mx3/cuda"
	"code.google.com/p/mx3/data"
)

// quantity that is not stored, but can output to (set) a buffer
type setter struct {
	nComp int
	mesh  *data.Mesh
	autosave
	setFn func(dst *data.Slice, good bool) // calculates quantity and stores in dst
}

func newSetter(nComp int, m *data.Mesh, name, unit string, setFunc func(dst *data.Slice, good bool)) *setter {
	return &setter{nComp, m, autosave{name: name}, setFunc}
}

// calculate quantity, save in dst, notify output may be needed
func (b *setter) set(dst *data.Slice, goodstep bool) {
	b.setFn(dst, goodstep)
	if goodstep && b.needSave() {
		goSaveCopy(b.autoFname(), dst, Time)
		b.saved()
	}
}

func (b *setter) Download() *data.Slice {
	buffer := cuda.GetBuffer(b.nComp, b.mesh)
	defer cuda.RecycleBuffer(buffer)
	b.set(buffer, false)
	return buffer.HostCopy()
}