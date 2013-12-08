package sshutil

import (
	"code.google.com/p/go.crypto/ssh"
	"log"
	"strings"
)

func Run(conn *ssh.ClientConn, cmd string) (string, error) {
	log.Println("[ssh]", "[run]", cmd)

	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	if strings.HasPrefix(cmd, "sudo") {
		err = session.RequestPty("xterm", 40, 80, ssh.TerminalModes{})
		if err != nil {
			return "", err
		}
	}

	bs, err := session.CombinedOutput(cmd)
	return string(bs), err
}
