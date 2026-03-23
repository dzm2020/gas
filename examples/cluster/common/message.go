package common

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Uid      int64  `json:"uid"`
}

type LoginResponse struct {
	Code int64 `json:"code"`
}

type ChatMessageRequest struct {
	Content string `json:"content"`
}

type ChatMessageResponse struct {
	Content string `json:"content"`
}
