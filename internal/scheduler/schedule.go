package scheduler

// ScheduleType is the type of schedule (start, stop, restart).
type ScheduleType int

const (
	ScheduleTypeStart ScheduleType = iota
	ScheduleTypeStop
	ScheduleTypeRestart
)

func (s ScheduleType) String() string {
	switch s {
	case ScheduleTypeStart:
		return "Start"

	case ScheduleTypeStop:
		return "Stop"

	case ScheduleTypeRestart:
		return "Restart"

	default:
		// Should never happen.
		return "Unknown"

	}
}
