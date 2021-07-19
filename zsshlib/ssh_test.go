package zsshlib

import (
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRemotePath(t *testing.T) {
	conn, _ := net.Dial("tcp", "localhost:3838")
	userHome, _ := os.UserHomeDir()
	factory := NewSshConfigFactoryImpl(getOsUser(), filepath.Join(userHome, SSH_DIR, ID_RSA))
	factory.port = 3838
	config := factory.Config()
	sshConn, _ := Dial(config, conn)

	client, _ := sftp.NewClient(sshConn)
	defer func() { _ = client.Close() }()

	result := checkRemotePath(client, "", "message.txt", false)
	assert.Equal(t, result, "message.txt", "Path not correct")

	result = checkRemotePath(client, "~", "message.txt", false)
	assert.Equal(t, result, "message.txt", "Path not correct")

	result = checkRemotePath(client, "/", "message.txt", false)
	assert.Equal(t, result, "message.txt", "Path not correct")

	result = checkRemotePath(client, "message.txt", "message.txt", false)
	assert.Equal(t, result, "message.txt", "Path not correct")
}
