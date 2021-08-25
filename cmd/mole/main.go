package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
)

/*
type Endpoint struct {
	Host string
	Port int
}

func (endpoint *Endpoint) String() string {
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}

type SSHtunnel struct {
	Local  *Endpoint
	Server *Endpoint
	Remote *Endpoint

	Config *ssh.ClientConfig
}

func (tunnel *SSHtunnel) Start() error {
	listener, err := net.Listen("tcp", tunnel.Local.String())
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go tunnel.forward(conn)
	}
}

func (tunnel *SSHtunnel) forward(localConn net.Conn) {
	serverConn, err := ssh.Dial("tcp", tunnel.Server.String(), tunnel.Config)
	if err != nil {
		fmt.Printf("Server dial error: %s\n", err)
		return
	}

	remoteConn, err := serverConn.Dial("tcp", tunnel.Remote.String())
	if err != nil {
		fmt.Printf("Remote dial error: %s\n", err)
		return
	}

	copyConn:=func(writer, reader net.Conn) {
		defer writer.Close()
		defer reader.Close()

		_, err:= io.Copy(writer, reader)
		if err != nil {
			fmt.Printf("io.Copy error: %s", err)
		}
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}

func SSHAgent() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}
func main() {
  localEndpoint := &Endpoint{
    Host: "localhost",
		Port: 9000,
	}

	serverEndpoint := &Endpoint{
    Host: "example.com",
		Port: 22,
	}

	remoteEndpoint := &Endpoint{
    Host: "localhost",
		Port: 8080,
	}

	sshConfig := &ssh.ClientConfig{
    User: "vcap",
		Auth: []ssh.AuthMethod{
      SSHAgent(),
		},
	}

	tunnel := &SSHtunnel{
    Config: sshConfig,
		Local:  localEndpoint,
		Server: serverEndpoint,
		Remote: remoteEndpoint,
	}

	tunnel.Start()
}
*/

// Flags: pflag --publish host:port --connect host:port --reconnect

var publish *string = flag.String("publish", "localhost:8080", "the local port you want to publish")
var connect *string = flag.String("connect", "server:1", "the local port you want to publish")
var server *string = flag.String("server", "lbsfilm.at", "the molehil server")
var serveruser *string = flag.String("user", "lukas", "the molehil server username")
var serverport *int = flag.Int("port", 2222, "the molehill servers port")

func main() {
	flag.Parse()
	fmt.Println("publish has value ", *publish)
	fmt.Println("connect has value ", *connect)

	cmd := exec.Command("ssh", "-N", "-L", *publish+":mole:1", *server, "-p", *serveruser+"@"+fmt.Sprint(*serverport))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	log.Fatal(cmd.Start())
}
