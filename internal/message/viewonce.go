package message

import "github.com/PeterStoica/chatwire/internal/wire"

func ViewOnceStub() *wire.Message {
	return &wire.Message{ViewOnceMessageV2: &wire.Message_FutureProofMessage{}}
}

func IsViewOnceStub(m *wire.Message) bool {
	v := m.GetViewOnceMessageV2()
	return v != nil && v.GetMessage() == nil
}
