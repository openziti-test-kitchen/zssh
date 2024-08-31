package main

import (
	"golang.org/x/crypto/ssh"
	"os"
)

func main() {
	key, _ := os.ReadFile(os.Args[1])
	signer, _ := ssh.ParsePrivateKey(key)
	config := &ssh.ClientConfig{
		User:            "ubuntu",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshClient, _ := ssh.Dial("tcp", os.Args[2], config)
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
