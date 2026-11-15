package model

// IdempotencyRecord представляет запись идемпотентности.
type IdempotencyRecord struct {
	Key          string
	RequestHash  string
	ResponseCode int
	ResponseBody []byte
}
