package atproto

type Identity struct {
	Handle string `json:"handle"`
	DID    string `json:"did"`
	PDS    string `json:"pds"`
}
