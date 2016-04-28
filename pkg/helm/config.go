package helm

import (
	"google.golang.org/grpc"
)

type config struct {
	ServAddr string
	Insecure bool
}

func (cfg *config) DialOpts() (opts []grpc.DialOption) {
	if cfg.Insecure {
		opts = append(opts, grpc.WithInsecure())
	} else {
		// TODO: handle transport credentials
	}

	return
}

func (cfg *config) CallOpts() (opts []grpc.CallOption) {
	return
}

func (cfg *config) client() *client {
	return &client{cfg: cfg}
}
