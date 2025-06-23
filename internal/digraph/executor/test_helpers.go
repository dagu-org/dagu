package executor

// testWriter is a simple writer for capturing output in tests
type testWriter struct {
	data []byte
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.data = append(tw.data, p...)
	return len(p), nil
}

func (tw *testWriter) String() string {
	return string(tw.data)
}

