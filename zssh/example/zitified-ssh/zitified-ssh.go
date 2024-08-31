package main

import (
	"net"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/openziti/sdk-golang/ziti"
)

func main() {
	key, _ := os.ReadFile(os.Args[1])
	signer, _ := ssh.ParsePrivateKey(key)
	config := &ssh.ClientConfig{
		User:            "ubuntu",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	//sshClient, _ := ssh.Dial("tcp", host, config)
	c, chans, reqs, _ := ssh.NewClientConn(obtainZitiConn(), "", config)
	sshClient := ssh.NewClient(c, chans, reqs)

	defer sshClient.Close()
	session, _ := sshClient.NewSession()
	defer session.Close()
	session.RequestPty("xterm", 80, 40, ssh.TerminalModes{})
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	session.Shell()
	session.Wait()
}

func obtainZitiConn() net.Conn {
	cfg, _ := ziti.NewConfigFromFile(os.Args[3])
	ctx, _ := ziti.NewContext(cfg)
	dialOptions := &ziti.DialOptions{
		Identity: os.Args[2],
	}
	c, _ := ctx.DialWithOptions("zsshSvc", dialOptions)
	return c
}
