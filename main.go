package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/jroimartin/gocui"
	"gitlab.com/tblyler/ep2pchat/client"
	"gitlab.com/tblyler/ep2pchat/server"
	"gitlab.com/tblyler/ep2pchat/signer"
)

func logErrConn(conn net.Conn, msgs ...interface{}) {
	fmt.Fprint(os.Stderr, "["+conn.RemoteAddr().String()+"] ")
	fmt.Fprintln(os.Stderr, msgs...)
}

func logConn(conn net.Conn, msgs ...interface{}) {
	fmt.Print("[" + conn.RemoteAddr().String() + "] ")
	fmt.Println(msgs...)
}

func logErr(msgs ...interface{}) {
	fmt.Fprintln(os.Stderr, msgs...)
}

func log(msgs ...interface{}) {
	fmt.Println(msgs...)
}

func main() {
	keyStr := ""
	host := "0.0.0.0:0"
	isServer := false

	flag.StringVar(&keyStr, "key", keyStr, "256-bit binary base64-encoded key value, generates otherwise")
	flag.StringVar(&host, "host", host, "the host to connect to or listen on")
	flag.BoolVar(&isServer, "server", isServer, "determines whether or not to act as a server")
	flag.Parse()

	err := func() (err error) {
		var sign signer.Signer
		if keyStr == "" {
			log("not using encryption nor signing")
			sign = signer.NewClearText()
		} else {
			key, err := base64.StdEncoding.DecodeString(keyStr)
			if err != nil || len(key) != signer.KeyLength {
				logErr("Failed to decode 256-bit key from base64")
				return err
			}

			var keyArray [signer.KeyLength]byte
			copy(keyArray[:], key)

			sign = signer.NewSecretBox(keyArray)
		}

		if isServer {
			listener, err := net.Listen("tcp", host)
			if err != nil {
				logErr("Failed to listen on address ", host)
				return err
			}

			defer listener.Close()

			log("Listening on ", listener.Addr().String())

			server := server.NewServer(listener, sign, context.Background())
			return server.Serve()
		}

		conn, err := net.Dial("tcp", host)
		if err != nil {
			logErr("Failed to connect to address ", host)
			return err
		}

		defer conn.Close()

		client := client.NewClient(conn, context.Background())

		gui, err := gocui.NewGui(gocui.OutputNormal)
		if err != nil {
			logErr("Failed to initialze UI")
			return err
		}
		defer gui.Close()

		gui.Cursor = true
		gui.SetManagerFunc(func(gui *gocui.Gui) error {
			maxX, maxY := gui.Size()

			v, err := gui.SetView("chat", 1, 1, maxX-1, maxY-5)
			if err != nil {
				if err != gocui.ErrUnknownView {
					return err
				}

				if _, err := gui.SetCurrentView("chat"); err != nil {
					return err
				}

				v.Editable = true
				v.Wrap = true
			}
			v, err = gui.SetView("input", 1, maxY-4, maxX-1, maxY-1)
			if err != nil {
				if err != gocui.ErrUnknownView {
					return err
				}

				if _, err := gui.SetCurrentView("input"); err != nil {
					return err
				}

				v.Editable = true
				v.Wrap = true
			}

			return nil
		})
		gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			return gocui.ErrQuit
		})
		gui.SetKeybinding("input", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			msg := v.ViewBuffer()

			if msg == "" {
				return nil
			}

			encMsg, err := sign.Encode([]byte(msg))
			if err != nil {
				logErr(err)
				return gocui.ErrQuit
			}

			err = client.SendMsg(encMsg)
			if err != nil {
				logErr(err)
				return gocui.ErrQuit
			}

			v.SetCursor(0, 0)
			v.Clear()

			return nil
		})

		go func() {
			for {
				encMsg, err := client.GetMsg()
				if err != nil {
					logErr(err)
					break
				}

				msg, err := sign.Decode(encMsg)
				if err != nil {
					logErr(err)
					break
				}

				gui.Update(func(gui *gocui.Gui) error {
					v, err := gui.View("chat")
					if err != nil {
						gui.Close()
						return err
					}

					_, err = io.Copy(v, bytes.NewReader(msg))
					if err != nil {
						logErr(err)
						return err
					}

					return nil
				})
				if err != nil {
					break
				}
			}

			gui.Close()
		}()

		err = gui.MainLoop()
		if err != nil && err != gocui.ErrQuit {
			return err
		}

		return nil
	}()

	if err != nil {
		logErr(err)
		os.Exit(1)
	}
}
