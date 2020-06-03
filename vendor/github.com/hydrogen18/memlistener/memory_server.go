package memlistener

import "net/http"

type MemoryServer struct {
	Listener *MemoryListener
	*http.Server
}

func NewInMemoryServer(h http.Handler) *MemoryServer {
	retval := &MemoryServer{}
	retval.Listener = NewMemoryListener()
	retval.Server = &http.Server{}
	retval.Handler = h
	go retval.Serve(retval.Listener)
	return retval
}

func (ms *MemoryServer) NewTransport() *http.Transport {
	transport := &http.Transport{}
	transport.Dial = ms.Listener.Dial
	return transport
}

func (ms *MemoryServer) NewClient() *http.Client {
	client := &http.Client{}
	client.Transport = ms.NewTransport()
	return client
}
