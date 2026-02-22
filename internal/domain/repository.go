package domain

// Repository represents the git repository being observed.
type Repository struct {
	Owner     string
	Name      string
	RemoteURL string
}
