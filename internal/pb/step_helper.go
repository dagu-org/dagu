package pb

import (
	"github.com/dagu-dev/dagu/internal/dag"
	// "google.golang.org/protobuf/encoding/protojson"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	structpb "google.golang.org/protobuf/types/known/structpb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

func ToDagStep(pbStep *Step) (*dag.Step, error) {
	if pbStep == nil {
		return nil, fmt.Errorf("pbStep must not be nil")
	}
	dagStep := &dag.Step{
		Name:         pbStep.Name,
		Description:  pbStep.Description,
		Variables:    pbStep.Variables,
		Dir:          pbStep.Dir,
		CmdWithArgs:  pbStep.CmdWithArgs,
		Command:      pbStep.Command,
		Script:       pbStep.Script,
		Stdout:       pbStep.Stdout,
		Stderr:       pbStep.Stderr,
		Output:       pbStep.Output,
		Args:         pbStep.Args,
		Depends:      pbStep.Depends,
		MailOnError:  pbStep.MailOnError,
		SignalOnStop: pbStep.SignalOnStop,
	}

	if pbStep.ExecutorConfig != nil {
		config := make(map[string]interface{}, len(pbStep.ExecutorConfig.Config))
		for k, v := range pbStep.ExecutorConfig.Config {
			vInterface, err := convertPbAnyToInterface(v)
			if err != nil {
				return nil, err
			}
			config[k] = vInterface
		}

		dagStep.ExecutorConfig = dag.ExecutorConfig{
			Type:   pbStep.ExecutorConfig.Type,
			Config: config,
		}
	}

	if pbStep.ContinueOn != nil {
		dagStep.ContinueOn = dag.ContinueOn{
			Failure: pbStep.ContinueOn.Failure,
			Skipped: pbStep.ContinueOn.Skipped,
		}
	}

	if pbStep.RetryPolicy != nil {
		dagStep.RetryPolicy = &dag.RetryPolicy{
			Limit:    int(pbStep.RetryPolicy.Limit),
			Interval: pbStep.RetryPolicy.Interval.AsDuration(),
		}
	}

	if pbStep.RepeatPolicy != nil {
		dagStep.RepeatPolicy = dag.RepeatPolicy{
			Repeat:   pbStep.RepeatPolicy.Repeat,
			Interval: pbStep.RepeatPolicy.Interval.AsDuration(),
		}
	}

	if pbStep.Preconditions != nil {
		conditions := make([]*dag.Condition, len(pbStep.Preconditions))
		for i, c := range pbStep.Preconditions {
			conditions[i] = &dag.Condition{
				Condition: c.Condition,
				Expected:  c.Expected,
			}
		}
		dagStep.Preconditions = conditions
	}

	return dagStep, nil
}

func ToPbStep(dagStep *dag.Step) (*Step, error) {
	if dagStep == nil {
		return nil, fmt.Errorf("dagStep must not be nil")
	}
	step := &Step{
		Name:         dagStep.Name,
		Description:  dagStep.Description,
		Variables:    dagStep.Variables,
		Dir:          dagStep.Dir,
		CmdWithArgs:  dagStep.CmdWithArgs,
		Command:      dagStep.Command,
		Script:       dagStep.Script,
		Stdout:       dagStep.Stdout,
		Stderr:       dagStep.Stderr,
		Output:       dagStep.Output,
		Args:         dagStep.Args,
		Depends:      dagStep.Depends,
		MailOnError:  dagStep.MailOnError,
		SignalOnStop: dagStep.SignalOnStop,
	}

	config := make(map[string]*anypb.Any, len(dagStep.ExecutorConfig.Config))
	for k, v := range dagStep.ExecutorConfig.Config {
		pMsg, err := convertToProtoMessage(v)
		if err != nil {
			return nil, err
		}

		any, err := anypb.New(pMsg)
		if err != nil {
			return nil, err
		}

		config[k] = any
	}
	step.ExecutorConfig = &ExecutorConfig{
		Type:   dagStep.ExecutorConfig.Type,
		Config: config,
	}

	step.ContinueOn = &ContinueOn{
		Failure: dagStep.ContinueOn.Failure,
		Skipped: dagStep.ContinueOn.Skipped,
	}

	if dagStep.RetryPolicy != nil {
		step.RetryPolicy = &RetryPolicy{
			Limit:    int32(dagStep.RetryPolicy.Limit),
			Interval: durationpb.New(dagStep.RetryPolicy.Interval),
		}
	}

	step.RepeatPolicy = &RepeatPolicy{
		Repeat:   dagStep.RepeatPolicy.Repeat,
		Interval: durationpb.New(dagStep.RepeatPolicy.Interval),
	}

	if dagStep.Preconditions != nil {
		conditions := make([]*Condition, len(dagStep.Preconditions))
		for i, c := range dagStep.Preconditions {
			conditions[i] = &Condition{
				Condition: c.Condition,
				Expected:  c.Expected,
			}
		}
		step.Preconditions = conditions
	}

	return step, nil
}

func convertPbAnyToInterface(any *anypb.Any) (interface{}, error) {
	switch any.TypeUrl {
	case "type.googleapis.com/google.protobuf.IntValue":
		var intValue wrapperspb.Int32Value
		if err := any.UnmarshalTo(&intValue); err != nil {
			return nil, fmt.Errorf("could not unmarshal IntValue: %w", err)
		}
		return intValue.GetValue(), nil
	case "type.googleapis.com/google.protobuf.StringValue":
		var stringValue wrapperspb.StringValue
		if err := any.UnmarshalTo(&stringValue); err != nil {
			return nil, fmt.Errorf("could not unmarshal StringValue: %w", err)
		}
		return stringValue.GetValue(), nil
	case "type.googleapis.com/google.protobuf.BoolValue":
		var boolValue wrapperspb.BoolValue
		if err := any.UnmarshalTo(&boolValue); err != nil {
			return nil, fmt.Errorf("could not unmarshal BoolValue: %w", err)
		}
		return boolValue.GetValue(), nil
	case "type.googleapis.com/google.protobuf.Struct":
		var structValue structpb.Struct
		if err := any.UnmarshalTo(&structValue); err != nil {
			return nil, fmt.Errorf("could not unmarshal Struct: %w", err)
		}
		return structValue.AsMap(), nil
	default:
		return nil, fmt.Errorf("unknown type URL: %s", any.TypeUrl)
	}
}

func convertToProtoMessage(v interface{}) (proto.Message, error) {
	switch value := v.(type) {
	case string:
		return wrapperspb.String(value), nil
	case int:
		return wrapperspb.Int32(int32(value)), nil
	case int32:
		return wrapperspb.Int32(value), nil
	case bool:
		return wrapperspb.Bool(value), nil
	case map[string]interface{}:
		return structpb.NewStruct(value)
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
}
