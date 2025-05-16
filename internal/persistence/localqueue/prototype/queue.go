package prototype

type Queue struct {
	// baseDir is the base directory for the queue files
	baseDir string
	// Prefix is the prefix for the queue files
	prefix string
}

func NewQueue(baseDir, prefix string) *Queue {
	return &Queue{
		baseDir: baseDir,
		prefix:  prefix,
	}
}
