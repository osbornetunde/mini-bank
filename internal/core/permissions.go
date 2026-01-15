package core

// Permission constants define all available permissions in the system.
// These are used for role-based access control (RBAC).
const (
	// Account permissions
	PermAccountsRead   = "accounts_read"
	PermAccountsWrite  = "accounts_write"
	PermAccountsUpdate = "accounts_update"

	// Transaction permissions
	PermTransactionsRead    = "transactions_read"
	PermTransactionsProcess = "transactions_process"

	// User permissions
	PermUsersRead   = "users_read"
	PermUsersWrite  = "users_write"
	PermUsersUpdate = "users_update"

	// Admin permissions
	PermPermissionsManage = "permissions_manage"

	// Fee permissions
	PermFeesManage = "fees_manage"
)

// AllPermissions returns a slice of all available permissions.
// Useful for validation and admin interfaces.
func AllPermissions() []string {
	return []string{
		PermAccountsRead,
		PermAccountsWrite,
		PermAccountsUpdate,
		PermTransactionsRead,
		PermTransactionsProcess,
		PermUsersRead,
		PermUsersWrite,
		PermUsersUpdate,
		PermPermissionsManage,
		PermFeesManage,
	}
}

// IsValidPermission checks if a permission string is valid.
func IsValidPermission(perm string) bool {
	for _, p := range AllPermissions() {
		if p == perm {
			return true
		}
	}
	return false
}
