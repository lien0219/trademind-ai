package settings

import (
	"strconv"
	"strings"
)

// DefaultWarningStockFromMap reads settings.inventory.default_warning_stock (fallback 5).
func DefaultWarningStockFromMap(m map[string]string) int {
	def := 5
	if m == nil {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(m["default_warning_stock"]))
	if err != nil || n < 0 {
		return def
	}
	return n
}

// DefaultSafetyStockFromMap reads settings.inventory.default_safety_stock (fallback 0).
func DefaultSafetyStockFromMap(m map[string]string) int {
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(m["default_safety_stock"]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// CoalesceDefaultStockLines clamps safety to warning for new SKU defaults.
func CoalesceDefaultStockLines(warning, safety int) (int, int) {
	if warning < 0 {
		warning = 5
	}
	if safety < 0 {
		safety = 0
	}
	if safety > warning {
		safety = warning
	}
	return warning, safety
}
