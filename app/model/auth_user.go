package model

type AuthUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	RoleID   int    `json:"role_id"`
	RoleName string `json:"role_name"`
}
