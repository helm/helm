package ping

type PingReq struct {
	RequestID string
}

type PingResponse struct {
	Status bool
	Data   string
}
