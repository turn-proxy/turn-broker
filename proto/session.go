package proto

type Session struct {
	RefreshedAt uint64   `json:"refreshed_at"`
	Username    string   `json:"username"`
	Credential  string   `json:"credential"`
	URLs        []string `json:"urls"`
	JoinLink    string   `json:"join_link"`
	ExpiresAt   uint64   `json:"expires_at"`
}
