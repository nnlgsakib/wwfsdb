package ipfsdb

import (
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ToAny converts an interface{} to a google.protobuf.Any
func ToAny(v interface{}) (*anypb.Any, error) {
	var any *anypb.Any
	var err error

	switch val := v.(type) {
	case int64:
		any, err = anypb.New(wrapperspb.Int64(val))
	case float64:
		any, err = anypb.New(wrapperspb.Double(val))
	case bool:
		any, err = anypb.New(wrapperspb.Bool(val))
	case string:
		any, err = anypb.New(wrapperspb.String(val))
	case []byte:
		any, err = anypb.New(wrapperspb.Bytes(val))
	default:
		err = fmt.Errorf("unsupported type for Any: %T", v)
	}

	return any, err
}

// FromAny converts a google.protobuf.Any to an interface{}
func FromAny(any *anypb.Any) (interface{}, error) {
	db, err := any.UnmarshalNew()
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Any: %w", err)
	}

	switch v := db.(type) {
	case *wrapperspb.Int64Value:
		return v.Value, nil
	case *wrapperspb.DoubleValue:
		return v.Value, nil
	case *wrapperspb.BoolValue:
		return v.Value, nil
	case *wrapperspb.StringValue:
		return v.Value, nil
	case *wrapperspb.BytesValue:
		return v.Value, nil
	default:
		return nil, fmt.Errorf("unsupported type in Any: %T", v)
	}
}
