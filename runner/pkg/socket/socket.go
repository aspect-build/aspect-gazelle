package socket

type Socket[S, R any] interface {
	Send(cmd S) error
	Recv() (R, error)
	Close() error
}

type Server[S, R any] interface {
	Socket[S, R]
	Serve(path string) error
	Accept() error
	HasConnection() bool
}
