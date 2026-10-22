package adminperm

import (
	"fmt"
	"regexp"
	"strings"
)

// Custom role codes are generated, immutable and cannot collide with system roles.
var customRolePattern = regexp.MustCompile(`^r_[a-f0-9]{24}$`)

func IsCustomRole(role string) bool {
	return customRolePattern.MatchString(strings.TrimSpace(strings.ToLower(role)))
}

// These privileges require a system administrator and cannot be delegated to
// self-defined roles; otherwise user-managers could mint new administrators.
var reservedCustomPermissions = map[string]bool{
	PermUserManage:             true,
	PermSettingsManage:         true,
	PermSecuritySessionManage:  true,
	PermPIIReadFull:            true,
	PermConfigManage:           true,
}

// ValidateCustomPermissions applies an explicit allowlist and never accepts
// unknown keys, duplicates, or reserved administrative privileges.
func ValidateCustomPermissions(perms []string) ([]string, error) {
	if len(perms) > len(allPermissions) {
		return nil, fmt.Errorf("权限数量超出上限")
	}
	allowed := make(map[string]bool, len(allPermissions))
	for _, k := range allPermissions {
		allowed[k] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(perms))
	for _, raw := range perms {
		k := strings.TrimSpace(raw)
		if !allowed[k] || reservedCustomPermissions[k] || seen[k] {
			return nil, fmt.Errorf("未知、重复或不可委派的权限: %s", k)
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}

// DelegablePermissions is returned to the role editor; system-only keys are
// never offered to custom role creators.
func DelegablePermissions() []string {
	out := make([]string, 0,len(allPermissions))
	for _, k := range allPermissions {
		if !reservedCustomPermissions[k] {
			out = append(out, k)
		}
	}
	return out
}
