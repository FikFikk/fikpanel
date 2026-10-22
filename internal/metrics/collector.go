package metrics

// Collector antarmuka pengumpulan metrik sistem lintas platform
type Collector interface {
	Collect() (Point, error)
}
