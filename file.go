package sshutil

import (
	"archive/tar"
	"code.google.com/p/go.crypto/ssh"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func cleanName(name string) string {
	return strings.Replace(name, " ", "\\ ", -1)
}

func getSha1(local string) (string, error) {
	f, err := os.Open(local)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	bs := h.Sum(nil)
	return hex.EncodeToString(bs), nil
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
		_, err = io.Copy(ww, f)
		if err != nil {
			done <- err
			return
		}
	}()

	<-hold

	go func() {
		log.Println("[ssh] [send-file]", local, "to", remote)
		done <- session.Run("mkdir -p " + path.Dir(cleanName(remote)) + " && tar -m -C " + path.Dir(cleanName(remote)) + " -x")
	}()

	return <-done
}

func GetLocalDigest(conn *ssh.ClientConn, folder string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.Walk(folder, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		h, err := getSha1(p)
		if err != nil {
			return err
		}

		f := strings.Replace(p[len(folder):], "\\", "/", -1)
		files[f] = h
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func GetRemoteDigest(conn *ssh.ClientConn, folder string) (map[string]string, error) {
	files := make(map[string]string)
	str, err := Run(conn, "find "+folder+" -type f -exec sha1sum {} \\;")
	if err != nil {
		return nil, err
	}
	for _, ln := range strings.Split(str, "\n") {
		idx := strings.IndexByte(ln, ' ')
		if idx > 0 {
			sha1sum := ln[:idx]
			for idx < len(ln)-1 && ln[idx] == ' ' {
				idx++
			}
			filename := ln[idx:]
			files[filename[len(folder):]] = sha1sum
		}
	}
	return files, nil
}

func SyncFolder(conn *ssh.ClientConn, local, remote string) error {
	localFiles, err := GetLocalDigest(conn, local)
	if err != nil {
		return err
	}

	remoteFiles, err := GetRemoteDigest(conn, remote)
	if err != nil {
		return err
	}

	Run(conn, "mkdir -p "+remote)

	// Delete files
	for f, _ := range remoteFiles {
		if _, ok := localFiles[f]; !ok {
			log.Println("[ssh] delete", remote+f)
			_, err = Run(conn, "rm "+remote+f)
			if err != nil {
				return err
			}
		}
	}

	// Add / Update files
	for f, lh := range localFiles {
		if rh, ok := remoteFiles[f]; !ok || rh != lh {
			log.Println("[ssh] upload", remote, f, rh, lh)
			err = SendFile(conn, filepath.Join(local, f[1:]), remote+f)
			if err != nil {
				return err
			}
		}
	}

	return err
}
