package ipfsdb

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
)

// ValidateAndCastValue checks if a value conforms to the specified SQL data type
// and returns the value cast to its corresponding Go type, wrapped in an *anypb.Any.
func ValidateAndCastValue(value string, dataType string) (*anypb.Any, error) {
	upperDataType := strings.ToUpper(dataType)
	var val interface{}
	var err error

	if strings.HasPrefix(upperDataType, "VARCHAR") {
		// For simplicity, we'll just check length for now if specified
		// Example: VARCHAR(255)
		var maxLen int = -1
		if _, errS := fmt.Sscanf(upperDataType, "VARCHAR(%d)", &maxLen); errS == nil {
			if len(value) > maxLen {
				return nil, fmt.Errorf("value '%s' exceeds maximum length of %d for type %s", value, maxLen, dataType)
			}
		}
		val = value
	} else {
		switch upperDataType {
		case "INT", "INTEGER":
			val, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid integer value: '%s'", value)
			}
		case "FLOAT", "REAL", "DOUBLE":
			val, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float value: '%s'", value)
			}
		case "BOOL", "BOOLEAN":
			val, err = strconv.ParseBool(strings.ToLower(value))
			if err != nil {
				return nil, fmt.Errorf("invalid boolean value: '%s'", value)
			}
		case "TEXT":
			val = value
		default:
			// For any other type not explicitly handled, treat as string for now.
			val = value
		}
	}

	if err != nil {
		return nil, err
	}

	return ToAny(val)
}
