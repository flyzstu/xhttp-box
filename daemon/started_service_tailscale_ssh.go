package daemon

import (
	E "github.com/sagernet/sing/common/exceptions"
	"google.golang.org/grpc"
)

func (s *StartedService) StartTailscaleSSHSession(
	server grpc.BidiStreamingServer[TailscaleSSHClientMessage, TailscaleSSHServerMessage],
) error {
	return E.New("Tailscale is removed in xhttp-box")
}
