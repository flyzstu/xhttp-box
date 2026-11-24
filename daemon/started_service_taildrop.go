package daemon

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *StartedService) SubscribeTaildropInbox(
	request *SubscribeTaildropInboxRequest,
	server grpc.ServerStreamingServer[TaildropInbox],
) error {
	return E.New("Taildrop is removed in xhttp-box")
}

func (s *StartedService) MarkTaildropInboxRead(ctx context.Context, request *MarkTaildropInboxReadRequest) (*emptypb.Empty, error) {
	return nil, E.New("Taildrop is removed in xhttp-box")
}

func (s *StartedService) SendTaildropFiles(
	server grpc.BidiStreamingServer[TaildropSendClientMessage, TaildropSendServerMessage],
) error {
	return E.New("Taildrop is removed in xhttp-box")
}

func (s *StartedService) DownloadTaildropFile(
	request *DownloadTaildropFileRequest,
	server grpc.ServerStreamingServer[DownloadTaildropFileChunk],
) error {
	return E.New("Taildrop is removed in xhttp-box")
}

func (s *StartedService) DeleteTaildropFile(ctx context.Context, request *DeleteTaildropFileRequest) (*emptypb.Empty, error) {
	return nil, E.New("Taildrop is removed in xhttp-box")
}

func (s *StartedService) CancelTaildropReceiving(ctx context.Context, request *CancelTaildropReceivingRequest) (*emptypb.Empty, error) {
	return nil, E.New("Taildrop is removed in xhttp-box")
}
