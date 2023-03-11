package admin

// func TestServeHTTP(t *testing.T) {
// 	h := &adminHandler{
// 		routes: map[string]map[*regexp.Regexp]http.HandlerFunc{},
// 	}
// 	fn := func(rw http.ResponseWriter, req *http.Request) {}
// 	h.addRoute(http.MethodGet, "/", fn)

// 	rw := &mockResponseWriter{}

// 	req := &http.Request{Method: http.MethodOptions}
// 	h.ServeHTTP(rw, req)
// 	require.Equal(t, http.StatusOK, rw.status)

// 	req = &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/"}}
// 	h.ServeHTTP(rw, req)
// 	require.Equal(t, http.StatusOK, rw.status)
// }

// type mockResponseWriter struct {
// 	status int
// 	body   string
// 	header *http.Header
// }

// var _ (http.ResponseWriter) = (*mockResponseWriter)(nil)

// func (h *mockResponseWriter) Header() http.Header {
// 	if h.header == nil {
// 		h.header = &http.Header{}
// 	}
// 	return *h.header
// }

// func (h *mockResponseWriter) Write(body []byte) (int, error) {
// 	h.body = string(body)
// 	return 0, nil
// }

// func (h *mockResponseWriter) WriteHeader(statusCode int) {
// 	h.status = statusCode
// }
