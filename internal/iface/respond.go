package iface

// NewErrorResponse 创建错误响应消息
func NewErrorResponse(errMsg string) *RespondMessage {
	return &RespondMessage{
		Error: errMsg,
	}
}

