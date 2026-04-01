package handler

// SuccessResponse is the standard API success envelope.
type SuccessResponse struct {
	Success bool        `json:"success" example:"true"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Error   string `json:"error"`
	Code    string `json:"code"`
	Detail  string `json:"detail,omitempty"`
}
