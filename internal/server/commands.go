package server

import (
	"errors"
	"log/slog"
	"net"
	"net/rpc"
	"sync"
	"time"
)

var registered sync.Once

type CommandHandler struct {
	rpcListener net.Listener
	router      *Router
}

type DeployArgs struct {
	Host              string
	TargetURL         string
	DeployTimeout     time.Duration
	DrainTimeout      time.Duration
	HealthCheckConfig HealthCheckConfig
	TargetOptions     TargetOptions
}

type PauseArgs struct {
	Host         string
	DrainTimeout time.Duration
	PauseTimeout time.Duration
}

type ResumeArgs struct {
	Host string
}

type RemoveArgs struct {
	Host string
}

type ListResponse struct {
	Targets map[string]string `json:"targets"`
}

func NewCommandHandler(router *Router) *CommandHandler {
	return &CommandHandler{
		router: router,
	}
}

func (h *CommandHandler) Start(socketPath string) error {
	var err error
	registered.Do(func() {
		err = rpc.RegisterName("parachute", h)
	})
	if err != nil {
		slog.Error("Failed to register RPC handler", "error", err)
		return err
	}

	h.rpcListener, err = net.Listen("unix", socketPath)
	if err != nil {
		slog.Error("Failed to start RPC listener", "error", err)
		return err
	}

	go func() {
		for {
			conn, err := h.rpcListener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					slog.Debug("Closing RPC listener")
					return
				} else {
					slog.Error("Error accepting RPC connection", "error", err)
					continue
				}
			}

			go rpc.ServeConn(conn)
		}
	}()

	return nil
}

func (h *CommandHandler) Stop() error {
	return h.rpcListener.Close()
}

func (h *CommandHandler) Deploy(args DeployArgs, reply *bool) error {
	target, err := NewTarget(args.TargetURL, args.HealthCheckConfig, args.TargetOptions)
	if err != nil {
		return err
	}

	err = h.router.SetServiceTarget(args.Host, target, args.DeployTimeout, args.DrainTimeout)

	return err
}

func (h *CommandHandler) Pause(args PauseArgs, reply *bool) error {
	err := h.router.PauseService(args.Host, args.DrainTimeout, args.PauseTimeout)

	return err
}

func (h *CommandHandler) Resume(args ResumeArgs, reply *bool) error {
	err := h.router.ResumeService(args.Host)

	return err
}

func (h *CommandHandler) Remove(args DeployArgs, reply *bool) error {
	err := h.router.RemoveService(args.Host)

	return err
}

func (h *CommandHandler) List(args bool, reply *ListResponse) error {
	reply.Targets = h.router.ListActiveServices()

	return nil
}
