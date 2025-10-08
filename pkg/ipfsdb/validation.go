package ipfsdb

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidateAndCastValue checks if a value conforms to the specified SQL data type
// and returns the value cast to its corresponding Go type.
func ValidateAndCastValue(value string, dataType string) (interface{}, error) {
	upperDataType := strings.ToUpper(dataType)

	if strings.HasPrefix(upperDataType, "VARCHAR") {
		// For simplicity, we'll just check length for now if specified
		// Example: VARCHAR(255)
		var maxLen int = -1
		if _, err := fmt.Sscanf(upperDataType, "VARCHAR(%d)", &maxLen); err == nil {
			if len(value) > maxLen {
				return nil, fmt.Errorf("value '%s' exceeds maximum length of %d for type %s", value, maxLen, dataType)
			}
		}
		return value, nil
	}

	switch upperDataType {
	case "INT", "INTEGER":
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer value: '%s'", value)
		}
		return i, nil
	case "FLOAT", "REAL", "DOUBLE":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float value: '%s'", value)
		}
		return f, nil
	case "BOOL", "BOOLEAN":
		b, err := strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return nil, fmt.Errorf("invalid boolean value: '%s'", value)
		}
		return b, nil
	case "TEXT":
		return value, nil
	default:
		// For any other type not explicitly handled, treat as string for now.
		// This maintains backward compatibility with unrecognized types.
		return value, nil
	}
}
