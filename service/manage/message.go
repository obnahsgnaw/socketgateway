package manage

const (
	Send    MsgType = 1
	Receive MsgType = 2
)

type Message struct {
	Fd      int
	Type    MsgType `json:"type"`
	Level   string  `json:"level"`
	Desc    string  `json:"desc"`
	Package []byte  `json:"package"`
}

type MsgType int

func (t MsgType) String() string {
	if t == Send {
		return "send"
	}
	return "received"
}
