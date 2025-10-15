package core

import (
	"fmt"
	"time"
)

// Helper function to convert value to string
func toString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	case fmt.Stringer:
		return val.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// Helper function to parse date/time strings
func parseDateTime(s string) (time.Time, error) {
	// Try common date/time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999",
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse datetime: %s", s)
}
