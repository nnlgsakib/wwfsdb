package ipfsdb

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
)

// ValidateAndCastValue checks if a value conforms to the specified SQL data type
// and returns the value cast to its corresponding Go type, wrapped in an *anypb.Any.
func ValidateAndCastValue(value string, dataType string) (*anypb.Any, error) {
	upperDataType := strings.ToUpper(dataType)
	var val interface{}
	var err error

	if value == "NULL" {
		return nil, nil // Represent SQL NULL as a nil Any pointer
	}

	if strings.HasPrefix(upperDataType, "VARCHAR") {
		var maxLen int = -1
		if _, errS := fmt.Sscanf(upperDataType, "VARCHAR(%d)", &maxLen); errS == nil {
			if len(value) > maxLen {
				return nil, fmt.Errorf("value '%s' exceeds maximum length of %d for type %s", value, maxLen, dataType)
			}
		}
		val = value
	} else if strings.HasPrefix(upperDataType, "DECIMAL") {
		// For now, treat DECIMAL as FLOAT for storage.
		val, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid decimal value: '%s'", value)
		}
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
		case "DATE":
			val, err = time.Parse("2006-01-02", value)
			if err != nil {
				return nil, fmt.Errorf("invalid date value: '%s', expected format YYYY-MM-DD", value)
			}
		case "TIME":
			val, err = time.Parse("15:04:05", value)
			if err != nil {
				return nil, fmt.Errorf("invalid time value: '%s', expected format HH:MM:SS", value)
			}
		case "TIMESTAMP":
			val, err = time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, fmt.Errorf("invalid timestamp value: '%s', expected format RFC3339 (e.g., 2006-01-02T15:04:05Z07:00)", value)
			}
		default:
			val = value
		}
	}

	if err != nil {
		return nil, err
	}

	return ToAny(val)
}
