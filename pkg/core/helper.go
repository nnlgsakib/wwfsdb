package core

import (
	"fmt"
	"time"
)

// Helper function to convert values to string
func toString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val), true
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val), true
	case float32, float64:
		return fmt.Sprintf("%v", val), true
	case bool:
		return fmt.Sprintf("%t", val), true
	case time.Time:
		return val.Format(time.RFC3339), true
	default:
		return fmt.Sprintf("%v", val), true
	}
}

// Helper function to parse date/time strings
func parseDateTime(s string) (time.Time, error) {
	// Try various date/time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
		"01/02/2006",
		"01-02-2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date/time: %s", s)
}
