package ws

type WSClient interface {
	GetSend() chan []byte
	WritePump()
}

type WSHub interface {
	Register(key string, c WSClient)
	Unregister(key string, c WSClient)
	Broadcast(key string, data []byte)
	Push(key string, data []byte)
}
