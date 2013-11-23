package sshutil

import (
	"code.google.com/p/go.crypto/ssh"
	"io"
	"io/ioutil"
)

type (
	KeyChain struct {
		keys []ssh.Signer
	}
)

func (k *KeyChain) Key(i int) (ssh.PublicKey, error) {
	if i < 0 || i >= len(k.keys) {
		return nil, nil
	}

	return k.keys[i].PublicKey(), nil
}

func (k *KeyChain) Sign(i int, rand io.Reader, data []byte) (sig []byte, err error) {
	return k.keys[i].Sign(rand, data)
}

func (k *KeyChain) LoadPEM(file string) error {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	key, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return err
	}
	k.keys = append(k.keys, key)
	return nil
}

func GetKeyChain(privateKeyFile string) (*KeyChain, error) {
	k := KeyChain{[]ssh.Signer{}}
	return &k, k.LoadPEM(privateKeyFile)
}
