package coordinator

import coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"

type Handler struct {
	coordinatorv1.UnimplementedCoordinatorServiceServer
}
