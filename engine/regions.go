package engine

import (
	"github.com/mumax/3/cuda"
	"github.com/mumax/3/data"
	"github.com/mumax/3/util"
	"log"
)

var regions = Regions{info: Info(1, "regions", "")} // global regions map

const NREGION = 256 // maximum number of regions. (!) duplicated in CUDA

func init() {
	DeclFunc("DefRegion", DefRegion, "Define a material region with given index (0-255) and shape")
	DeclFunc("DefRegionCell", DefRegionCell, "Set a material region in one cell by index")
	DeclROnly("regions", &regions, "Outputs the region index for each cell")
}

// stores the region index for each cell
type Regions struct {
	gpuCache *cuda.Bytes  // TODO: rename: buffer
	deflist  []regionHist // history
	info
}

// keeps history of region definitions
type regionHist struct {
	region int
	shape  Shape
}

func (r *Regions) alloc() {
	log.Println("regions.alloc")
	mesh := r.Mesh()
	r.gpuCache = cuda.NewBytes(mesh.NCell())
	DefRegion(0, universe)
}

func (r *Regions) resize() {
	newSize := Mesh().Size()
	r.gpuCache.Free()
	r.gpuCache = cuda.NewBytes(prod(newSize))
	log.Println("regions.re-alloc")
	for _, d := range r.deflist {
		r.render(d.region, d.shape)
	}
}

// Define a region with id (0-255) to be inside the Shape.
func DefRegion(id int, s Shape) {
	defRegionId(id)
	regions.render(id, s)
	regions.deflist = append(regions.deflist, regionHist{id, s})
}

// renders (rasterizes) shape, filling it with region number #id, between x1 and x2
// TODO: a tidbit expensive
func (r *Regions) render(id int, s Shape) {
	log.Println("regions.render", id, s)
	n := Mesh().Size()
	l := r.HostList() // need to start from previous state
	arr := reshapeBytes(l, r.Mesh().Size())

	for iz := 0; iz < n[Z]; iz++ {
		for iy := 0; iy < n[Y]; iy++ {
			for ix := 0; ix < n[X]; ix++ {
				r := Index2Coord(ix, iy, iz)
				if s(r[X], r[Y], r[Z]) { // inside
					arr[iz][iy][ix] = byte(id)
				}
			}
		}
	}
	log.Print("regions.upload")
	r.gpuCache.Upload(l)
}

func (r *Regions) get(R data.Vector) int {
	for i := len(r.deflist) - 1; i >= 0; i-- {
		d := r.deflist[i]
		if d.shape(R[X], R[Y], R[Z]) {
			return d.region
		}
	}
	return 0
}

func (r *Regions) HostArray() [][][]byte {
	return reshapeBytes(r.HostList(), r.Mesh().Size())
}

func (r *Regions) HostList() []byte {
	regionsList := make([]byte, r.Mesh().NCell())
	regions.gpuCache.Download(regionsList)
	return regionsList
}

func DefRegionCell(id int, x, y, z int) {
	defRegionId(id)
	index := data.Index(Mesh().Size(), z, y, x)
	regions.gpuCache.Set(index, byte(id))
}

func defRegionId(id int) {
	if id < 0 || id > NREGION {
		util.Fatalf("region id should be 0 -%v, have: %v", NREGION, id)
	}
	checkMesh()
}

// normalized volume (0..1) of region.
// TODO: a tidbit too expensive
func (r *Regions) volume(region_ int) float64 {
	region := byte(region_)
	vol := 0
	list := r.HostList()
	for _, reg := range list {
		if reg == region {
			vol++
		}
	}
	V := float64(vol) / float64(r.Mesh().NCell())
	return V
}

// Set the region of one cell
func (r *Regions) SetCell(ix, iy, iz int, region int) {
	size := Mesh().Size()
	i := data.Index(size, ix, iy, iz)
	r.gpuCache.Set(i, byte(region))
}

// Get the region data on GPU
func (r *Regions) Gpu() *cuda.Bytes {
	return r.gpuCache
}

var unitMap inputParam // unit map used to output regions quantity

func init() {
	unitMap.init(1, "unit", "", nil)
	for r := 0; r < NREGION; r++ {
		unitMap.setRegion(r, []float64{float64(r)})
	}
}

// Get returns the regions as a slice of floats, so it can be output.
func (r *Regions) Slice() (*data.Slice, bool) {
	buf := cuda.Buffer(1, r.Mesh().Size())
	cuda.RegionDecode(buf, unitMap.gpuLUT1(), regions.Gpu())
	return buf, true
}

// Re-interpret a contiguous array as a multi-dimensional array of given size.
func reshapeBytes(array []byte, size [3]int) [][][]byte {
	Nx, Ny, Nz := size[X], size[Y], size[Z]
	util.Argument(Nx*Ny*Nz == len(array))
	sliced := make([][][]byte, Nz)
	for i := range sliced {
		sliced[i] = make([][]byte, Ny)
	}
	for i := range sliced {
		for j := range sliced[i] {
			sliced[i][j] = array[(i*Ny+j)*Nx+0 : (i*Ny+j)*Nx+Nx]
		}
	}
	return sliced
}

func (b *Regions) shift(dx int) {
	// TODO: return if no regions defined
	r1 := b.Gpu()
	r2 := cuda.NewBytes(b.Mesh().NCell()) // TODO: somehow recycle
	defer r2.Free()
	newreg := byte(0) // new region at edge
	cuda.ShiftBytes(r2, r1, b.Mesh(), dx, newreg)
	r1.Copy(r2)

	n := Mesh().Size()
	x1, x2 := shiftDirtyRange(dx)

	for iz := 0; iz < n[Z]; iz++ {
		for iy := 0; iy < n[Y]; iy++ {
			for ix := x1; ix < x2; ix++ {
				r := Index2Coord(ix, iy, iz) // includes shift
				reg := b.get(r)
				if reg != 0 {
					b.SetCell(ix, iy, iz, reg) // a bit slowish, but hardly reached
				}
			}
		}
	}
}

func (r *Regions) Mesh() *data.Mesh { return Mesh() }

func prod(s [3]int) int {
	return s[0] * s[1] * s[2]
}
