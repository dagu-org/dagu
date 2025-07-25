// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.31.1
// source: proto/coordinator/v1/coordinator.proto

package coordinatorv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Operation int32

const (
	Operation_OPERATION_UNSPECIFIED Operation = 0
	Operation_OPERATION_START       Operation = 1 // Start a new DAG run
	Operation_OPERATION_RETRY       Operation = 2 // Retry an existing run
)

// Enum value maps for Operation.
var (
	Operation_name = map[int32]string{
		0: "OPERATION_UNSPECIFIED",
		1: "OPERATION_START",
		2: "OPERATION_RETRY",
	}
	Operation_value = map[string]int32{
		"OPERATION_UNSPECIFIED": 0,
		"OPERATION_START":       1,
		"OPERATION_RETRY":       2,
	}
)

func (x Operation) Enum() *Operation {
	p := new(Operation)
	*p = x
	return p
}

func (x Operation) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Operation) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_coordinator_v1_coordinator_proto_enumTypes[0].Descriptor()
}

func (Operation) Type() protoreflect.EnumType {
	return &file_proto_coordinator_v1_coordinator_proto_enumTypes[0]
}

func (x Operation) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Operation.Descriptor instead.
func (Operation) EnumDescriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{0}
}

// Health status of a worker based on heartbeat recency.
type WorkerHealthStatus int32

const (
	WorkerHealthStatus_WORKER_HEALTH_STATUS_UNSPECIFIED WorkerHealthStatus = 0
	WorkerHealthStatus_WORKER_HEALTH_STATUS_HEALTHY     WorkerHealthStatus = 1 // Last heartbeat < 5 seconds
	WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING     WorkerHealthStatus = 2 // Last heartbeat 5-15 seconds
	WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY   WorkerHealthStatus = 3 // Last heartbeat > 15 seconds
)

// Enum value maps for WorkerHealthStatus.
var (
	WorkerHealthStatus_name = map[int32]string{
		0: "WORKER_HEALTH_STATUS_UNSPECIFIED",
		1: "WORKER_HEALTH_STATUS_HEALTHY",
		2: "WORKER_HEALTH_STATUS_WARNING",
		3: "WORKER_HEALTH_STATUS_UNHEALTHY",
	}
	WorkerHealthStatus_value = map[string]int32{
		"WORKER_HEALTH_STATUS_UNSPECIFIED": 0,
		"WORKER_HEALTH_STATUS_HEALTHY":     1,
		"WORKER_HEALTH_STATUS_WARNING":     2,
		"WORKER_HEALTH_STATUS_UNHEALTHY":   3,
	}
)

func (x WorkerHealthStatus) Enum() *WorkerHealthStatus {
	p := new(WorkerHealthStatus)
	*p = x
	return p
}

func (x WorkerHealthStatus) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (WorkerHealthStatus) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_coordinator_v1_coordinator_proto_enumTypes[1].Descriptor()
}

func (WorkerHealthStatus) Type() protoreflect.EnumType {
	return &file_proto_coordinator_v1_coordinator_proto_enumTypes[1]
}

func (x WorkerHealthStatus) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use WorkerHealthStatus.Descriptor instead.
func (WorkerHealthStatus) EnumDescriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{1}
}

// Request message for polling a task.
type PollRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	WorkerId      string                 `protobuf:"bytes,1,opt,name=worker_id,json=workerId,proto3" json:"worker_id,omitempty"`
	PollerId      string                 `protobuf:"bytes,2,opt,name=poller_id,json=pollerId,proto3" json:"poller_id,omitempty"`                                                       // Unique ID for this poll request
	Labels        map[string]string      `protobuf:"bytes,3,rep,name=labels,proto3" json:"labels,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"` // Worker labels for task matching
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PollRequest) Reset() {
	*x = PollRequest{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PollRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PollRequest) ProtoMessage() {}

func (x *PollRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PollRequest.ProtoReflect.Descriptor instead.
func (*PollRequest) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{0}
}

func (x *PollRequest) GetWorkerId() string {
	if x != nil {
		return x.WorkerId
	}
	return ""
}

func (x *PollRequest) GetPollerId() string {
	if x != nil {
		return x.PollerId
	}
	return ""
}

func (x *PollRequest) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

// Response message for polling a task.
type PollResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Task          *Task                  `protobuf:"bytes,1,opt,name=task,proto3" json:"task,omitempty"` // The task to process.
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PollResponse) Reset() {
	*x = PollResponse{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PollResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PollResponse) ProtoMessage() {}

func (x *PollResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PollResponse.ProtoReflect.Descriptor instead.
func (*PollResponse) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{1}
}

func (x *PollResponse) GetTask() *Task {
	if x != nil {
		return x.Task
	}
	return nil
}

// Request message for dispatching a task.
type DispatchRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Task          *Task                  `protobuf:"bytes,1,opt,name=task,proto3" json:"task,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DispatchRequest) Reset() {
	*x = DispatchRequest{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DispatchRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DispatchRequest) ProtoMessage() {}

func (x *DispatchRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DispatchRequest.ProtoReflect.Descriptor instead.
func (*DispatchRequest) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{2}
}

func (x *DispatchRequest) GetTask() *Task {
	if x != nil {
		return x.Task
	}
	return nil
}

// Response message for dispatching a task.
type DispatchResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DispatchResponse) Reset() {
	*x = DispatchResponse{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DispatchResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DispatchResponse) ProtoMessage() {}

func (x *DispatchResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DispatchResponse.ProtoReflect.Descriptor instead.
func (*DispatchResponse) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{3}
}

// Task to process.
type Task struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	Operation        Operation              `protobuf:"varint,6,opt,name=operation,proto3,enum=coordinator.v1.Operation" json:"operation,omitempty"`
	RootDagRunName   string                 `protobuf:"bytes,1,opt,name=root_dag_run_name,json=rootDagRunName,proto3" json:"root_dag_run_name,omitempty"`
	RootDagRunId     string                 `protobuf:"bytes,2,opt,name=root_dag_run_id,json=rootDagRunId,proto3" json:"root_dag_run_id,omitempty"`
	ParentDagRunName string                 `protobuf:"bytes,3,opt,name=parent_dag_run_name,json=parentDagRunName,proto3" json:"parent_dag_run_name,omitempty"`
	ParentDagRunId   string                 `protobuf:"bytes,4,opt,name=parent_dag_run_id,json=parentDagRunId,proto3" json:"parent_dag_run_id,omitempty"`
	DagRunId         string                 `protobuf:"bytes,5,opt,name=dag_run_id,json=dagRunId,proto3" json:"dag_run_id,omitempty"`
	Target           string                 `protobuf:"bytes,7,opt,name=target,proto3" json:"target,omitempty"`                                                                                                                  // DAG name or path
	Params           string                 `protobuf:"bytes,8,opt,name=params,proto3" json:"params,omitempty"`                                                                                                                  // Optional: parameters
	Step             string                 `protobuf:"bytes,9,opt,name=step,proto3" json:"step,omitempty"`                                                                                                                      // Optional: specific step (for RETRY)
	WorkerSelector   map[string]string      `protobuf:"bytes,10,rep,name=worker_selector,json=workerSelector,proto3" json:"worker_selector,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"` // Required worker labels for execution
	Definition       string                 `protobuf:"bytes,11,opt,name=definition,proto3" json:"definition,omitempty"`                                                                                                         // Optional: DAG definition (YAML) for local DAGs
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Task) Reset() {
	*x = Task{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Task) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Task) ProtoMessage() {}

func (x *Task) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Task.ProtoReflect.Descriptor instead.
func (*Task) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{4}
}

func (x *Task) GetOperation() Operation {
	if x != nil {
		return x.Operation
	}
	return Operation_OPERATION_UNSPECIFIED
}

func (x *Task) GetRootDagRunName() string {
	if x != nil {
		return x.RootDagRunName
	}
	return ""
}

func (x *Task) GetRootDagRunId() string {
	if x != nil {
		return x.RootDagRunId
	}
	return ""
}

func (x *Task) GetParentDagRunName() string {
	if x != nil {
		return x.ParentDagRunName
	}
	return ""
}

func (x *Task) GetParentDagRunId() string {
	if x != nil {
		return x.ParentDagRunId
	}
	return ""
}

func (x *Task) GetDagRunId() string {
	if x != nil {
		return x.DagRunId
	}
	return ""
}

func (x *Task) GetTarget() string {
	if x != nil {
		return x.Target
	}
	return ""
}

func (x *Task) GetParams() string {
	if x != nil {
		return x.Params
	}
	return ""
}

func (x *Task) GetStep() string {
	if x != nil {
		return x.Step
	}
	return ""
}

func (x *Task) GetWorkerSelector() map[string]string {
	if x != nil {
		return x.WorkerSelector
	}
	return nil
}

func (x *Task) GetDefinition() string {
	if x != nil {
		return x.Definition
	}
	return ""
}

// Request message for getting workers.
type GetWorkersRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetWorkersRequest) Reset() {
	*x = GetWorkersRequest{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetWorkersRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetWorkersRequest) ProtoMessage() {}

func (x *GetWorkersRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetWorkersRequest.ProtoReflect.Descriptor instead.
func (*GetWorkersRequest) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{5}
}

// Response message for getting workers.
type GetWorkersResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Workers       []*WorkerInfo          `protobuf:"bytes,1,rep,name=workers,proto3" json:"workers,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetWorkersResponse) Reset() {
	*x = GetWorkersResponse{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetWorkersResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetWorkersResponse) ProtoMessage() {}

func (x *GetWorkersResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetWorkersResponse.ProtoReflect.Descriptor instead.
func (*GetWorkersResponse) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{6}
}

func (x *GetWorkersResponse) GetWorkers() []*WorkerInfo {
	if x != nil {
		return x.Workers
	}
	return nil
}

// Information about a worker and its pollers.
type WorkerInfo struct {
	state       protoimpl.MessageState `protogen:"open.v1"`
	WorkerId    string                 `protobuf:"bytes,1,opt,name=worker_id,json=workerId,proto3" json:"worker_id,omitempty"`
	PollerId    string                 `protobuf:"bytes,2,opt,name=poller_id,json=pollerId,proto3" json:"poller_id,omitempty"` // Deprecated: Only used for backward compatibility
	Labels      map[string]string      `protobuf:"bytes,3,rep,name=labels,proto3" json:"labels,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	ConnectedAt int64                  `protobuf:"varint,4,opt,name=connected_at,json=connectedAt,proto3" json:"connected_at,omitempty"` // Unix timestamp in seconds
	// Aggregated stats from heartbeat
	TotalPollers    int32              `protobuf:"varint,5,opt,name=total_pollers,json=totalPollers,proto3" json:"total_pollers,omitempty"`
	BusyPollers     int32              `protobuf:"varint,6,opt,name=busy_pollers,json=busyPollers,proto3" json:"busy_pollers,omitempty"`
	RunningTasks    []*RunningTask     `protobuf:"bytes,7,rep,name=running_tasks,json=runningTasks,proto3" json:"running_tasks,omitempty"`
	LastHeartbeatAt int64              `protobuf:"varint,8,opt,name=last_heartbeat_at,json=lastHeartbeatAt,proto3" json:"last_heartbeat_at,omitempty"`                             // Unix timestamp of last heartbeat
	HealthStatus    WorkerHealthStatus `protobuf:"varint,9,opt,name=health_status,json=healthStatus,proto3,enum=coordinator.v1.WorkerHealthStatus" json:"health_status,omitempty"` // Health status based on heartbeat recency
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *WorkerInfo) Reset() {
	*x = WorkerInfo{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *WorkerInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WorkerInfo) ProtoMessage() {}

func (x *WorkerInfo) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WorkerInfo.ProtoReflect.Descriptor instead.
func (*WorkerInfo) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{7}
}

func (x *WorkerInfo) GetWorkerId() string {
	if x != nil {
		return x.WorkerId
	}
	return ""
}

func (x *WorkerInfo) GetPollerId() string {
	if x != nil {
		return x.PollerId
	}
	return ""
}

func (x *WorkerInfo) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *WorkerInfo) GetConnectedAt() int64 {
	if x != nil {
		return x.ConnectedAt
	}
	return 0
}

func (x *WorkerInfo) GetTotalPollers() int32 {
	if x != nil {
		return x.TotalPollers
	}
	return 0
}

func (x *WorkerInfo) GetBusyPollers() int32 {
	if x != nil {
		return x.BusyPollers
	}
	return 0
}

func (x *WorkerInfo) GetRunningTasks() []*RunningTask {
	if x != nil {
		return x.RunningTasks
	}
	return nil
}

func (x *WorkerInfo) GetLastHeartbeatAt() int64 {
	if x != nil {
		return x.LastHeartbeatAt
	}
	return 0
}

func (x *WorkerInfo) GetHealthStatus() WorkerHealthStatus {
	if x != nil {
		return x.HealthStatus
	}
	return WorkerHealthStatus_WORKER_HEALTH_STATUS_UNSPECIFIED
}

// Request message for heartbeat.
type HeartbeatRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	WorkerId      string                 `protobuf:"bytes,1,opt,name=worker_id,json=workerId,proto3" json:"worker_id,omitempty"`
	Labels        map[string]string      `protobuf:"bytes,2,rep,name=labels,proto3" json:"labels,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	Stats         *WorkerStats           `protobuf:"bytes,3,opt,name=stats,proto3" json:"stats,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HeartbeatRequest) Reset() {
	*x = HeartbeatRequest{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HeartbeatRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HeartbeatRequest) ProtoMessage() {}

func (x *HeartbeatRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HeartbeatRequest.ProtoReflect.Descriptor instead.
func (*HeartbeatRequest) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{8}
}

func (x *HeartbeatRequest) GetWorkerId() string {
	if x != nil {
		return x.WorkerId
	}
	return ""
}

func (x *HeartbeatRequest) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *HeartbeatRequest) GetStats() *WorkerStats {
	if x != nil {
		return x.Stats
	}
	return nil
}

// Response message for heartbeat.
type HeartbeatResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HeartbeatResponse) Reset() {
	*x = HeartbeatResponse{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HeartbeatResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HeartbeatResponse) ProtoMessage() {}

func (x *HeartbeatResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HeartbeatResponse.ProtoReflect.Descriptor instead.
func (*HeartbeatResponse) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{9}
}

// Worker statistics reported via heartbeat.
type WorkerStats struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	TotalPollers  int32                  `protobuf:"varint,1,opt,name=total_pollers,json=totalPollers,proto3" json:"total_pollers,omitempty"` // Total number of pollers
	BusyPollers   int32                  `protobuf:"varint,2,opt,name=busy_pollers,json=busyPollers,proto3" json:"busy_pollers,omitempty"`    // Number currently processing tasks
	RunningTasks  []*RunningTask         `protobuf:"bytes,3,rep,name=running_tasks,json=runningTasks,proto3" json:"running_tasks,omitempty"`  // Details of running tasks
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *WorkerStats) Reset() {
	*x = WorkerStats{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *WorkerStats) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WorkerStats) ProtoMessage() {}

func (x *WorkerStats) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WorkerStats.ProtoReflect.Descriptor instead.
func (*WorkerStats) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{10}
}

func (x *WorkerStats) GetTotalPollers() int32 {
	if x != nil {
		return x.TotalPollers
	}
	return 0
}

func (x *WorkerStats) GetBusyPollers() int32 {
	if x != nil {
		return x.BusyPollers
	}
	return 0
}

func (x *WorkerStats) GetRunningTasks() []*RunningTask {
	if x != nil {
		return x.RunningTasks
	}
	return nil
}

// Information about a running task.
type RunningTask struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	DagRunId         string                 `protobuf:"bytes,1,opt,name=dag_run_id,json=dagRunId,proto3" json:"dag_run_id,omitempty"`
	DagName          string                 `protobuf:"bytes,2,opt,name=dag_name,json=dagName,proto3" json:"dag_name,omitempty"`
	StartedAt        int64                  `protobuf:"varint,3,opt,name=started_at,json=startedAt,proto3" json:"started_at,omitempty"` // Unix timestamp in seconds
	RootDagRunName   string                 `protobuf:"bytes,4,opt,name=root_dag_run_name,json=rootDagRunName,proto3" json:"root_dag_run_name,omitempty"`
	RootDagRunId     string                 `protobuf:"bytes,5,opt,name=root_dag_run_id,json=rootDagRunId,proto3" json:"root_dag_run_id,omitempty"`
	ParentDagRunName string                 `protobuf:"bytes,6,opt,name=parent_dag_run_name,json=parentDagRunName,proto3" json:"parent_dag_run_name,omitempty"`
	ParentDagRunId   string                 `protobuf:"bytes,7,opt,name=parent_dag_run_id,json=parentDagRunId,proto3" json:"parent_dag_run_id,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *RunningTask) Reset() {
	*x = RunningTask{}
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RunningTask) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunningTask) ProtoMessage() {}

func (x *RunningTask) ProtoReflect() protoreflect.Message {
	mi := &file_proto_coordinator_v1_coordinator_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunningTask.ProtoReflect.Descriptor instead.
func (*RunningTask) Descriptor() ([]byte, []int) {
	return file_proto_coordinator_v1_coordinator_proto_rawDescGZIP(), []int{11}
}

func (x *RunningTask) GetDagRunId() string {
	if x != nil {
		return x.DagRunId
	}
	return ""
}

func (x *RunningTask) GetDagName() string {
	if x != nil {
		return x.DagName
	}
	return ""
}

func (x *RunningTask) GetStartedAt() int64 {
	if x != nil {
		return x.StartedAt
	}
	return 0
}

func (x *RunningTask) GetRootDagRunName() string {
	if x != nil {
		return x.RootDagRunName
	}
	return ""
}

func (x *RunningTask) GetRootDagRunId() string {
	if x != nil {
		return x.RootDagRunId
	}
	return ""
}

func (x *RunningTask) GetParentDagRunName() string {
	if x != nil {
		return x.ParentDagRunName
	}
	return ""
}

func (x *RunningTask) GetParentDagRunId() string {
	if x != nil {
		return x.ParentDagRunId
	}
	return ""
}

var File_proto_coordinator_v1_coordinator_proto protoreflect.FileDescriptor

const file_proto_coordinator_v1_coordinator_proto_rawDesc = "" +
	"\n" +
	"&proto/coordinator/v1/coordinator.proto\x12\x0ecoordinator.v1\"\xc3\x01\n" +
	"\vPollRequest\x12\x1b\n" +
	"\tworker_id\x18\x01 \x01(\tR\bworkerId\x12\x1b\n" +
	"\tpoller_id\x18\x02 \x01(\tR\bpollerId\x12?\n" +
	"\x06labels\x18\x03 \x03(\v2'.coordinator.v1.PollRequest.LabelsEntryR\x06labels\x1a9\n" +
	"\vLabelsEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"8\n" +
	"\fPollResponse\x12(\n" +
	"\x04task\x18\x01 \x01(\v2\x14.coordinator.v1.TaskR\x04task\";\n" +
	"\x0fDispatchRequest\x12(\n" +
	"\x04task\x18\x01 \x01(\v2\x14.coordinator.v1.TaskR\x04task\"\x12\n" +
	"\x10DispatchResponse\"\x83\x04\n" +
	"\x04Task\x127\n" +
	"\toperation\x18\x06 \x01(\x0e2\x19.coordinator.v1.OperationR\toperation\x12)\n" +
	"\x11root_dag_run_name\x18\x01 \x01(\tR\x0erootDagRunName\x12%\n" +
	"\x0froot_dag_run_id\x18\x02 \x01(\tR\frootDagRunId\x12-\n" +
	"\x13parent_dag_run_name\x18\x03 \x01(\tR\x10parentDagRunName\x12)\n" +
	"\x11parent_dag_run_id\x18\x04 \x01(\tR\x0eparentDagRunId\x12\x1c\n" +
	"\n" +
	"dag_run_id\x18\x05 \x01(\tR\bdagRunId\x12\x16\n" +
	"\x06target\x18\a \x01(\tR\x06target\x12\x16\n" +
	"\x06params\x18\b \x01(\tR\x06params\x12\x12\n" +
	"\x04step\x18\t \x01(\tR\x04step\x12Q\n" +
	"\x0fworker_selector\x18\n" +
	" \x03(\v2(.coordinator.v1.Task.WorkerSelectorEntryR\x0eworkerSelector\x12\x1e\n" +
	"\n" +
	"definition\x18\v \x01(\tR\n" +
	"definition\x1aA\n" +
	"\x13WorkerSelectorEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"\x13\n" +
	"\x11GetWorkersRequest\"J\n" +
	"\x12GetWorkersResponse\x124\n" +
	"\aworkers\x18\x01 \x03(\v2\x1a.coordinator.v1.WorkerInfoR\aworkers\"\xe3\x03\n" +
	"\n" +
	"WorkerInfo\x12\x1b\n" +
	"\tworker_id\x18\x01 \x01(\tR\bworkerId\x12\x1b\n" +
	"\tpoller_id\x18\x02 \x01(\tR\bpollerId\x12>\n" +
	"\x06labels\x18\x03 \x03(\v2&.coordinator.v1.WorkerInfo.LabelsEntryR\x06labels\x12!\n" +
	"\fconnected_at\x18\x04 \x01(\x03R\vconnectedAt\x12#\n" +
	"\rtotal_pollers\x18\x05 \x01(\x05R\ftotalPollers\x12!\n" +
	"\fbusy_pollers\x18\x06 \x01(\x05R\vbusyPollers\x12@\n" +
	"\rrunning_tasks\x18\a \x03(\v2\x1b.coordinator.v1.RunningTaskR\frunningTasks\x12*\n" +
	"\x11last_heartbeat_at\x18\b \x01(\x03R\x0flastHeartbeatAt\x12G\n" +
	"\rhealth_status\x18\t \x01(\x0e2\".coordinator.v1.WorkerHealthStatusR\fhealthStatus\x1a9\n" +
	"\vLabelsEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"\xe3\x01\n" +
	"\x10HeartbeatRequest\x12\x1b\n" +
	"\tworker_id\x18\x01 \x01(\tR\bworkerId\x12D\n" +
	"\x06labels\x18\x02 \x03(\v2,.coordinator.v1.HeartbeatRequest.LabelsEntryR\x06labels\x121\n" +
	"\x05stats\x18\x03 \x01(\v2\x1b.coordinator.v1.WorkerStatsR\x05stats\x1a9\n" +
	"\vLabelsEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"\x13\n" +
	"\x11HeartbeatResponse\"\x97\x01\n" +
	"\vWorkerStats\x12#\n" +
	"\rtotal_pollers\x18\x01 \x01(\x05R\ftotalPollers\x12!\n" +
	"\fbusy_pollers\x18\x02 \x01(\x05R\vbusyPollers\x12@\n" +
	"\rrunning_tasks\x18\x03 \x03(\v2\x1b.coordinator.v1.RunningTaskR\frunningTasks\"\x91\x02\n" +
	"\vRunningTask\x12\x1c\n" +
	"\n" +
	"dag_run_id\x18\x01 \x01(\tR\bdagRunId\x12\x19\n" +
	"\bdag_name\x18\x02 \x01(\tR\adagName\x12\x1d\n" +
	"\n" +
	"started_at\x18\x03 \x01(\x03R\tstartedAt\x12)\n" +
	"\x11root_dag_run_name\x18\x04 \x01(\tR\x0erootDagRunName\x12%\n" +
	"\x0froot_dag_run_id\x18\x05 \x01(\tR\frootDagRunId\x12-\n" +
	"\x13parent_dag_run_name\x18\x06 \x01(\tR\x10parentDagRunName\x12)\n" +
	"\x11parent_dag_run_id\x18\a \x01(\tR\x0eparentDagRunId*P\n" +
	"\tOperation\x12\x19\n" +
	"\x15OPERATION_UNSPECIFIED\x10\x00\x12\x13\n" +
	"\x0fOPERATION_START\x10\x01\x12\x13\n" +
	"\x0fOPERATION_RETRY\x10\x02*\xa2\x01\n" +
	"\x12WorkerHealthStatus\x12$\n" +
	" WORKER_HEALTH_STATUS_UNSPECIFIED\x10\x00\x12 \n" +
	"\x1cWORKER_HEALTH_STATUS_HEALTHY\x10\x01\x12 \n" +
	"\x1cWORKER_HEALTH_STATUS_WARNING\x10\x02\x12\"\n" +
	"\x1eWORKER_HEALTH_STATUS_UNHEALTHY\x10\x032\xcd\x02\n" +
	"\x12CoordinatorService\x12A\n" +
	"\x04Poll\x12\x1b.coordinator.v1.PollRequest\x1a\x1c.coordinator.v1.PollResponse\x12M\n" +
	"\bDispatch\x12\x1f.coordinator.v1.DispatchRequest\x1a .coordinator.v1.DispatchResponse\x12S\n" +
	"\n" +
	"GetWorkers\x12!.coordinator.v1.GetWorkersRequest\x1a\".coordinator.v1.GetWorkersResponse\x12P\n" +
	"\tHeartbeat\x12 .coordinator.v1.HeartbeatRequest\x1a!.coordinator.v1.HeartbeatResponseB=Z;github.com/dagu-org/dagu/proto/coordinator/v1;coordinatorv1b\x06proto3"

var (
	file_proto_coordinator_v1_coordinator_proto_rawDescOnce sync.Once
	file_proto_coordinator_v1_coordinator_proto_rawDescData []byte
)

func file_proto_coordinator_v1_coordinator_proto_rawDescGZIP() []byte {
	file_proto_coordinator_v1_coordinator_proto_rawDescOnce.Do(func() {
		file_proto_coordinator_v1_coordinator_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proto_coordinator_v1_coordinator_proto_rawDesc), len(file_proto_coordinator_v1_coordinator_proto_rawDesc)))
	})
	return file_proto_coordinator_v1_coordinator_proto_rawDescData
}

var file_proto_coordinator_v1_coordinator_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_proto_coordinator_v1_coordinator_proto_msgTypes = make([]protoimpl.MessageInfo, 16)
var file_proto_coordinator_v1_coordinator_proto_goTypes = []any{
	(Operation)(0),             // 0: coordinator.v1.Operation
	(WorkerHealthStatus)(0),    // 1: coordinator.v1.WorkerHealthStatus
	(*PollRequest)(nil),        // 2: coordinator.v1.PollRequest
	(*PollResponse)(nil),       // 3: coordinator.v1.PollResponse
	(*DispatchRequest)(nil),    // 4: coordinator.v1.DispatchRequest
	(*DispatchResponse)(nil),   // 5: coordinator.v1.DispatchResponse
	(*Task)(nil),               // 6: coordinator.v1.Task
	(*GetWorkersRequest)(nil),  // 7: coordinator.v1.GetWorkersRequest
	(*GetWorkersResponse)(nil), // 8: coordinator.v1.GetWorkersResponse
	(*WorkerInfo)(nil),         // 9: coordinator.v1.WorkerInfo
	(*HeartbeatRequest)(nil),   // 10: coordinator.v1.HeartbeatRequest
	(*HeartbeatResponse)(nil),  // 11: coordinator.v1.HeartbeatResponse
	(*WorkerStats)(nil),        // 12: coordinator.v1.WorkerStats
	(*RunningTask)(nil),        // 13: coordinator.v1.RunningTask
	nil,                        // 14: coordinator.v1.PollRequest.LabelsEntry
	nil,                        // 15: coordinator.v1.Task.WorkerSelectorEntry
	nil,                        // 16: coordinator.v1.WorkerInfo.LabelsEntry
	nil,                        // 17: coordinator.v1.HeartbeatRequest.LabelsEntry
}
var file_proto_coordinator_v1_coordinator_proto_depIdxs = []int32{
	14, // 0: coordinator.v1.PollRequest.labels:type_name -> coordinator.v1.PollRequest.LabelsEntry
	6,  // 1: coordinator.v1.PollResponse.task:type_name -> coordinator.v1.Task
	6,  // 2: coordinator.v1.DispatchRequest.task:type_name -> coordinator.v1.Task
	0,  // 3: coordinator.v1.Task.operation:type_name -> coordinator.v1.Operation
	15, // 4: coordinator.v1.Task.worker_selector:type_name -> coordinator.v1.Task.WorkerSelectorEntry
	9,  // 5: coordinator.v1.GetWorkersResponse.workers:type_name -> coordinator.v1.WorkerInfo
	16, // 6: coordinator.v1.WorkerInfo.labels:type_name -> coordinator.v1.WorkerInfo.LabelsEntry
	13, // 7: coordinator.v1.WorkerInfo.running_tasks:type_name -> coordinator.v1.RunningTask
	1,  // 8: coordinator.v1.WorkerInfo.health_status:type_name -> coordinator.v1.WorkerHealthStatus
	17, // 9: coordinator.v1.HeartbeatRequest.labels:type_name -> coordinator.v1.HeartbeatRequest.LabelsEntry
	12, // 10: coordinator.v1.HeartbeatRequest.stats:type_name -> coordinator.v1.WorkerStats
	13, // 11: coordinator.v1.WorkerStats.running_tasks:type_name -> coordinator.v1.RunningTask
	2,  // 12: coordinator.v1.CoordinatorService.Poll:input_type -> coordinator.v1.PollRequest
	4,  // 13: coordinator.v1.CoordinatorService.Dispatch:input_type -> coordinator.v1.DispatchRequest
	7,  // 14: coordinator.v1.CoordinatorService.GetWorkers:input_type -> coordinator.v1.GetWorkersRequest
	10, // 15: coordinator.v1.CoordinatorService.Heartbeat:input_type -> coordinator.v1.HeartbeatRequest
	3,  // 16: coordinator.v1.CoordinatorService.Poll:output_type -> coordinator.v1.PollResponse
	5,  // 17: coordinator.v1.CoordinatorService.Dispatch:output_type -> coordinator.v1.DispatchResponse
	8,  // 18: coordinator.v1.CoordinatorService.GetWorkers:output_type -> coordinator.v1.GetWorkersResponse
	11, // 19: coordinator.v1.CoordinatorService.Heartbeat:output_type -> coordinator.v1.HeartbeatResponse
	16, // [16:20] is the sub-list for method output_type
	12, // [12:16] is the sub-list for method input_type
	12, // [12:12] is the sub-list for extension type_name
	12, // [12:12] is the sub-list for extension extendee
	0,  // [0:12] is the sub-list for field type_name
}

func init() { file_proto_coordinator_v1_coordinator_proto_init() }
func file_proto_coordinator_v1_coordinator_proto_init() {
	if File_proto_coordinator_v1_coordinator_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proto_coordinator_v1_coordinator_proto_rawDesc), len(file_proto_coordinator_v1_coordinator_proto_rawDesc)),
			NumEnums:      2,
			NumMessages:   16,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_coordinator_v1_coordinator_proto_goTypes,
		DependencyIndexes: file_proto_coordinator_v1_coordinator_proto_depIdxs,
		EnumInfos:         file_proto_coordinator_v1_coordinator_proto_enumTypes,
		MessageInfos:      file_proto_coordinator_v1_coordinator_proto_msgTypes,
	}.Build()
	File_proto_coordinator_v1_coordinator_proto = out.File
	file_proto_coordinator_v1_coordinator_proto_goTypes = nil
	file_proto_coordinator_v1_coordinator_proto_depIdxs = nil
}
