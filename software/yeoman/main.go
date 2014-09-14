package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"io"
	"log"
	"net"
	"path/filepath"

	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.crypto/ssh/terminal"
)

func main() {
	LoadUsers()
	return

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{

		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in
			// a production setting.
			if c.User() == "tony" && string(pass) == "tiger" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	privateBytes, err := ioutil.ReadFile("credentials/id_rsa")
	if err != nil {
		panic("Failed to load private key")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		panic("Failed to parse private key")
	}

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:2022")
	if err != nil {
		panic("failed to listen for connection")
	}
	log.Printf("Listening on port 2022")

	for {

		nConn, err := listener.Accept()
		if err != nil {
			panic("failed to accept incoming connection")
		}
		//TODO: Log connection information

		// Before use, a handshake must be performed on the incoming
		// net.Conn.
		_, chans, reqs, err := ssh.NewServerConn(nConn, config)
		if err != nil {
			log.Printf("Connection negotiation failed")
			continue
		}
		// The incoming Request channel must be serviced.
		go handleConnection(chans, reqs)
	}
}

func LoadUsers() (keys []ssh.PublicKey) {
	//keydir := "credentials/users"
	
	matches, err := filepath.Glob("credentials/users/*.pub")
	if err != nil {
		log.Fatal("Could not match", err)
	}

	for match := range matches {
		log.Println(match)
	}
	return nil
}

func handleConnection(chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Negotiation failed: Channel Accept", err)
			return
		}
		go shellHandler(channel, requests)
	}

}

func serveTerminal(t *terminal.Terminal, done chan bool) {
	for {
		input, err := t.ReadLine()
		if err != nil {
			//TODO handle EOF
			if err != io.EOF {
				log.Println("Unexpected connection termination", err)
			}
			break
		}
		log.Printf("Got input [%s]\n", input)

		data := "User 15\n\rUser16\n\rUser17\n\r"
		_, err = t.Write([]byte(data))
		if err != nil {
			log.Println("Client connection lost", err)
			break
		}
	}
	log.Println("term done")
	close(done)

}

func parsePtyRequest(s []byte) (width, height int, ok bool) {
	_, s, ok = parseString(s)
	if !ok {
		log.Println("pty parse fail")
		return
	}

	width32, s, ok := parseUint32(s)
	if !ok {
		log.Println("pty parse fail1")
	}
	height32, s, ok := parseUint32(s)
	if !ok {
		log.Println("pty parse fail2")
	}

	width = int(width32)
	height = int(height32)
	return
}

func parseWinchRequest(s []byte) (width, height int, ok bool) {
	width32, s, ok := parseUint32(s)
	if !ok {
		return
	}
	height32, s, ok := parseUint32(s)
	if !ok {
		return
	}

	width = int(width32)
	height = int(height32)
	if width < 1 { width = 1}
	if height < 1 {height = 1}
	return
}

/*
* Spawn two coroutines to handle a 'session' type SSH connection.
* The first thread handles all out-of-band SSH requests.
* The second represents the actual terminal.
 */
func shellHandler(ch ssh.Channel, in <-chan *ssh.Request) {
	//Default terminal sizes
	ptyW := 80
	ptyH := 20

	defer ch.Close()
	var term *terminal.Terminal

	//How the terminal coroutine tells us we're done
	isDone := make(chan bool)

	inClosed, shellClosed := false, false

	//Service all control-channel requests
	//For standard SSH connections, we'll get these three:
	// pty
	// env
	// shell
	//
	// PTY indicates we should allocate a pty. env is ignored,
	// then shell enables shell mode
	// The shell handling coroutine also spawns a channel to
	// tell us it is done.

	for {
		if inClosed || shellClosed {
			break
		}
		select {
		case req, inOpen := <-in:
			if !inOpen {
				inClosed = true
				break
			}
			ok := false

			switch req.Type {
			//Switch to shell mode
			case "shell":
				if term != nil {
					ok = true
					go serveTerminal(term, isDone)
				}
			case "env":
				ok = true
			//Allocate a PTY. The only data we snarf from
			//the request is the dimensions.
			case "pty-req":
				ptyW, ptyH, ok = parsePtyRequest(req.Payload)
				term = terminal.NewTerminal(ch, "golang> ")
				err := term.SetSize(ptyW, ptyH)
				if err != nil {
					ok = false
				}

			//This can arrive out-of-band, during the existing session
			//It indicates we should change the window size.
			case "window-change":
				ptyW, ptyH, ok = parseWinchRequest(req.Payload)
				err := term.SetSize(ptyW, ptyH)
				if err != nil {
					ok = false
				}
			}
			if req.WantReply {
				req.Reply(ok, nil)
			}
		case _, shellOpen := <-isDone:
			if !shellOpen {
				shellClosed = true
				break
			}

		}
	}
	log.Println("Connection closed")//TODO IP
}

func myDiscardRequests(in <-chan *ssh.Request) {
	for req := range in {
		log.Println("Got a global request", req.Type)
		if req.WantReply {
			req.Reply(false, nil)
		}
	}
}

func parseString(in []byte) (out string, rest []byte, ok bool) {
	if len(in) < 4 {
		return
	}
	length := binary.BigEndian.Uint32(in)
	if uint32(len(in)) < 4+length {
		return
	}
	out = string(in[4 : 4+length])
	rest = in[4+length:]
	ok = true
	return
}

func parseUint32(in []byte) (uint32, []byte, bool) {
	if len(in) < 4 {
		return 0, nil, false
	}
	return binary.BigEndian.Uint32(in), in[4:], true
}
