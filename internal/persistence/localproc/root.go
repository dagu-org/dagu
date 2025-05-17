package localproc

type Root struct {
	baseDir string
}

func NewRoot(baseDir string) *Root {
	return &Root{
		baseDir: baseDir,
	}
}
