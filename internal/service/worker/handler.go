package worker

import workerservicev1 "github.com/dagu-org/dagu/proto/core/workerservice/v1"

type Handler struct {
	workerservicev1.UnimplementedWorkerServiceServer
}
