package engine

type Factory interface {
	Create() Engine
}

type factoryImpl struct {
}

func NewFactory() Factory {
	return &factoryImpl{}
}

func (f *factoryImpl) Create() Engine {
	return New()
}
