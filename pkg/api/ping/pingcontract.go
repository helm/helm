package ping

type Req struct {
	RequestID string
}

type Response struct {
	Status bool
	Data   string
}
