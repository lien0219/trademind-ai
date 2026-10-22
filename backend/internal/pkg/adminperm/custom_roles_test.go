package adminperm

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestCustomRolePermissionValidation(t *testing.T) {
	code := "r_0123456789abcdef01234567"
	require.True(t,IsCustomRole(code))
	require.False(t,IsCustomRole("r_custom"))
	require.False(t,IsCustomRole("admin"))
	grants,err := ValidateCustomPermissions([]string{PermCustomerView, PermCustomerOperate})
	require.NoError(t,err)
	p := &Principal{Role:code,Permissions:grants}
	require.True(t,p.Can(PermCustomerOperate))
	require.False(t,p.Can(PermUserManage))
	require.False(t,p.IsAdmin())
	require.False(t,p.IsReadonly())
	_,err=ValidateCustomPermissions([]string{PermUserManage})
	require.Error(t,err)
	_,err=ValidateCustomPermissions([]string{PermCustomerView,PermCustomerView})
	require.Error(t,err)
	_,err=ValidateCustomPermissions([]string{"invalid.permission"})
	require.Error(t,err)
	require.Empty(t,PermissionsForRole(code))
	require.False(t,(&Principal{Role:"unknown",Permissions:[]string{PermUserManage}}).Can(PermUserManage))
}
