package sshutil

import (
	"bufio"
	"code.google.com/p/go.crypto/ssh"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type (
	configFileOption struct {
		key, value string
	}
	configFileHost struct {
		pattern *regexp.Regexp
		options []configFileOption
	}
	configFile struct {
		hosts []configFileHost
	}
)

func parsePattern(pattern string) (*regexp.Regexp, error) {
	pattern = strings.Replace(pattern, ".", "\\.", -1)
	pattern = strings.Replace(pattern, "*", ".*", -1)
	pattern = strings.Replace(pattern, "?", ".?", -1)
	return regexp.Compile(pattern)
}

func parseConfigFile(rd io.Reader) (*configFile, error) {
	scanner := bufio.NewScanner(rd)

	cfg := configFile{[]configFileHost{}}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) == 2 {
			fields[1] = strings.TrimFunc(fields[1], func(r rune) bool {
				return r == '"'
			})

			if fields[0] == "Host" {
				pattern, err := parsePattern(fields[1])
				if err != nil {
					return nil, err
				}

				cfg.hosts = append(cfg.hosts, configFileHost{
					pattern: pattern,
					options: []configFileOption{},
				})
			} else if len(cfg.hosts) > 0 {
				cfg.hosts[len(cfg.hosts)-1].options = append(
					cfg.hosts[len(cfg.hosts)-1].options,
					configFileOption{fields[0], fields[1]},
				)
			}
		}
	}

	return &cfg, nil
}

func (this *configFile) getOptions(hostname string) []configFileOption {
	for _, host := range this.hosts {
		if host.pattern.MatchString(hostname) {
			return host.options
		}
	}
	return []configFileOption{}
}

func getUserName(options []configFileOption) string {
	for _, kv := range options {
		if kv.key == "User" {
			return kv.value
		}
	}

	u, err := user.Current()
	if err != nil {
		return "root"
	} else {
		return u.Name
	}
}

func homeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

// The file name may use the tilde syntax to refer to a user's home
// directory or one of the following escape characters:
// `%d' (local user's home directory),
// `%u' (local user name),
// `%l' (local host name),
// `%h' (remote host name) or
// `%r' (remote user name).
func toAbsolute(remoteHostName, remoteUserName, dir string) string {
	if strings.Contains(dir, "~") {
		dir = strings.Replace(dir, "~", homeDir(), -1)
	}
	if strings.Contains(dir, "%d") {
		dir = strings.Replace(dir, "%d", homeDir(), -1)
	}
	if strings.Contains(dir, "%u") {
		var localUserName string
		u, err := user.Current()
		if err == nil {
			localUserName = u.Name
		}
		dir = strings.Replace(dir, "%u", localUserName, -1)
	}
	if strings.Contains(dir, "%l") {
		localHostName, _ := os.Hostname()
		dir = strings.Replace(dir, "%l", localHostName, -1)
	}
	if strings.Contains(dir, "%h") {
		dir = strings.Replace(dir, "%h", remoteHostName, -1)
	}
	if strings.Contains(dir, "%r") {
		dir = strings.Replace(dir, "%r", remoteUserName, -1)
	}
	return dir
}

// Dial a hostname, intelligently using your local ssh settings to do so
func Dial(hostname string) (*ssh.ClientConn, error) {
	options := []configFileOption{}

	f, err := os.Open(filepath.Join(homeDir(), ".ssh", "config"))
	if err == nil {
		defer f.Close()
		cfg, err := parseConfigFile(f)
		if err == nil {
			options = cfg.getOptions(hostname)
		}
	}

	port := 22
	username := getUserName(options)
	for _, kv := range options {
		switch kv.key {
		case "HostName":
			hostname = kv.value
		case "Port":
			port, _ = strconv.Atoi(kv.value)
		}
	}

	auths := []ssh.ClientAuth{}
	for _, kv := range options {
		switch kv.key {
		case "IdentityFile":
			keychain, err := GetKeyChain(toAbsolute(hostname, username, kv.value))
			if err == nil {
				auths = append(auths, ssh.ClientAuthKeyring(keychain))
			}
		}
	}

	log.Println("Dialing ", hostname, port, username, auths)

	return ssh.Dial("tcp", fmt.Sprint(hostname, ":", port), &ssh.ClientConfig{
		User: username,
		Auth: auths,
	})
}
