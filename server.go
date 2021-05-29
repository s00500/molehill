package main

import (
	"io"

	log "github.com/s00500/env_logger"

	"github.com/gliderlabs/ssh"
)

//go:generate sh injectGitVars.sh

func main() {
	log.Info("Starting ssh server version %s (%s)", gitTag, gitRevision)

	//forwardHandler := &ForwardedTCPHandler{}
	forwardHandler := &ssh.ForwardedTCPHandler{}

	server := ssh.Server{
		Addr: ":2222",
		PasswordHandler: ssh.PasswordHandler(func(ctx ssh.Context, password string) bool {
			return password == "qelruvqeuvqeriucnmqercmiqerlicmeroicmercerc"
		}),
		Handler: ssh.Handler(func(s ssh.Session) {
			io.WriteString(s, "Only remote forwarding available...\n")
		}),
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
			log.Println("attempt to GRAB", destinationHost, destinationPort, "for user", ctx.User(), ctx.RemoteAddr(), "granted")
			return true
		}),
		// What the providers do:
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			log.Println("attempt to BIND", host, port, "for user", ctx.User(), ctx.RemoteAddr(), "granted")
			return true
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
		},
	}

	server.SetOption(ssh.HostKeyFile("../hostkeys/server_id_rsa"))

	log.Fatal(server.ListenAndServe())
}
