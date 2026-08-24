package protocol

import (
	"net/http"

	"github.com/webxsid/pdg/internal/protocol/atproto"
)

type Protocols struct {
	ATProto *atproto.Client
}

func New(
	httpClient *http.Client,
) *Protocols {
	return &Protocols{
		ATProto: atproto.NewClient(httpClient),
	}
}
