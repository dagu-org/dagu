package admin

// func requestLogger(next http.Handler) http.Handler {
// 	return http.HandlerFunc(
// 		func(w http.ResponseWriter, r *http.Request) {
// 			log.Printf("Request received: %v %s %s",
// 				r.RemoteAddr, r.Method, r.URL.Path)
// 			next.ServeHTTP(w, r)
// 		})
// }
