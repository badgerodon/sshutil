package sshutil

import (
	"archive/tar"
	"code.google.com/p/go.crypto/ssh"
	"io"
	"log"
	"os"
	"path"
	"strings"
)

func cleanName(name string) string {
	return strings.Replace(name, " ", "\\ ", -1)
}

func SendFile(conn *ssh.ClientConn, local, remote string) error {
	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}

	hold := make(chan bool)
	done := make(chan error)

	go func() {
		w, err := session.StdinPipe()
		if err != nil {
			done <- err
			return
		}
		defer w.Close()

		ww := tar.NewWriter(w)
		defer ww.Close()

		hold <- true

		hdr, err := tar.FileInfoHeader(info, "/"+path.Base(remote))
		if err != nil {
			done <- err
			return
		}
		hdr.Name = path.Base(remote)
		err = ww.WriteHeader(hdr)
		if err != nil {
			done <- err
			return
		}
		n, err := io.Copy(ww, f)
		if err != nil {
			done <- err
			return
		}
		log.Println("- wrote", n)
	}()

	<-hold

	go func() {
		log.Println("uploading ", local, " to ", remote)
		done <- session.Run("mkdir -p " + path.Dir(cleanName(remote)) + " && tar -C " + path.Dir(cleanName(remote)) + " -x")
	}()

	return <-done
}
