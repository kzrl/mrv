// Package utils provides shared helper functions for mrv.
package utils

// OrDefault returns val if it is non-empty, otherwise returns def.
func OrDefault(val, def string) string {
	if val != "" {
		return val
	}
	return def
}
