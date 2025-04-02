package persistence

type Run struct {
	File   string
	Status Status
}

func NewRun(file string, status Status) *Run {
	return &Run{
		File:   file,
		Status: status,
	}
}
