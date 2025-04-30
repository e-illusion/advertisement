package webutil

import (
	"encoding/json"
	"net/http"
)

type Response struct {
    Message string      `json:"message,omitempty"`
    Error   string      `json:"error,omitempty"`
    Data    interface{} `json:"data,omitempty"`
}

// --- 新增：辅助函数，用于发送 JSON 响应 ---
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload) // 理论上应该处理 Marshal 的错误，但通常结构体转换不会失败

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// --- 新增：辅助函数，用于发送错误 JSON 响应 ---
func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, Response{Error: message})
}