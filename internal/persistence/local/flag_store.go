package local

import "github.com/dagu-dev/dagu/internal/persistence"

type flagStoreImpl struct {
	suspendedFlagsDir string
}

func (f flagStoreImpl) ToggleSuspend(id string, suspend bool) error {
	//TODO implement me
	panic("implement me")
}

func (f flagStoreImpl) IsSuspended(id string) bool {
	//TODO implement me
	panic("implement me")
}

func NewFlagStore(suspendedFlagsDir string) persistence.FlagStore {
	return &flagStoreImpl{
		suspendedFlagsDir: suspendedFlagsDir,
	}
}
