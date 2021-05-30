package main

import (
	"fmt"
	"io"
	"molehill/filehandlers"
	"net"
	"os"
	"sync"

	log "github.com/s00500/env_logger"
	"github.com/s00500/store"

	"github.com/gliderlabs/ssh"
)

//go:generate sh injectGitVars.sh

type Config struct {
	Runaddress string
	Users      []UserConfig
}

type UserConfig struct {
	Name            string
	Password        string
	PublicKey       string
	AllowedBinds    []string
	AllowedConnects []string
}

var configMu sync.RWMutex
var config Config = Config{
	Runaddress: ":2222",
	Users: []UserConfig{
		{
			Name:            "lukas",
			Password:        "lukas", // empty means autogenerate ? not sure
			AllowedBinds:    []string{"localhost:1"},
			AllowedConnects: []string{"localhost:1"},
		},
	},
}

func main() {
	log.Infof("Starting ssh server version %s (%s)", gitTag, gitRevision)
	os.RemoveAll("sockets")
	log.MustFatal(os.MkdirAll("sockets", 0755))

	// Load config
	configMu.Lock()
	store.Load("config.yml", &config)
	configMu.Unlock()

	//forwardHandler := &ForwardedTCPHandler{}
	forwardHandler := &filehandlers.ForwardedTCPToFileHandler{}

	server := ssh.Server{
		Addr: config.Runaddress,
		PublicKeyHandler: ssh.PublicKeyHandler(func(ctx ssh.Context, key ssh.PublicKey) bool {
			configMu.RLock()
			for _, user := range config.Users {
				if user.Name == ctx.User() {
					if user.PublicKey == "" {
						continue
					}

					userKey, err := ssh.ParsePublicKey([]byte(user.PublicKey))
					if !log.Should(err) {
						configMu.RUnlock()
						return ssh.KeysEqual(userKey, key)
					}
				}
			}
			configMu.RUnlock()
			return false
		}),
		PasswordHandler: ssh.PasswordHandler(func(ctx ssh.Context, password string) bool {
			configMu.RLock()
			for _, user := range config.Users {
				if user.Name == ctx.User() {
					if user.Password == "" {
						continue
					}
					configMu.RUnlock()
					return user.Password == password
				}
			}
			configMu.RUnlock()
			return false
		}),
		Handler: ssh.Handler(func(s ssh.Session) {
			io.WriteString(s, "Only remote forwarding available...\n")
		}),
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
			log.Println("attempt to GRAB", destinationHost, destinationPort, "for user", ctx.User(), ctx.RemoteAddr(), "granted")

			configMu.RLock()
			for _, user := range config.Users {
				if user.Name != ctx.User() {
					continue
				}

				for _, allowed := range user.AllowedConnects {
					if allowed == net.JoinHostPort(destinationHost, fmt.Sprint(destinationPort)) {
						configMu.RUnlock()
						return true
					}
				}
				break
			}
			configMu.RUnlock()
			return false
		}),
		// What the providers do:
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			log.Println("attempt to BIND", host, port, "for user", ctx.User(), ctx.RemoteAddr(), "...")

			configMu.RLock()
			for _, user := range config.Users {
				if user.Name != ctx.User() {
					continue
				}

				for _, allowed := range user.AllowedBinds {
					if allowed == net.JoinHostPort(host, fmt.Sprint(port)) {
						configMu.RUnlock()
						return true
					}
				}
				break
			}
			configMu.RUnlock()
			return false
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": filehandlers.DirectTCPIPHandler,
		},
	}

	server.SetOption(ssh.HostKeyFile("hostkeys/server_id_rsa"))

	log.Fatal(server.ListenAndServe())
}
