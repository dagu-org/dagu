package persistence

type Execution struct {
	File   string
	Status Status
}

func NewExecution(file string, status Status) *Execution {
	return &Execution{
		File:   file,
		Status: status,
	}
}
