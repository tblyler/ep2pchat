package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"sync"

	"gitlab.com/tblyler/ep2pchat/client"
	"gitlab.com/tblyler/ep2pchat/signer"
)

type Server struct {
	listener net.Listener
	signer   signer.Signer
	ctx      context.Context

	clients     map[client.ClientID]*client.Client
	clientsLock sync.Mutex
	msgChan     chan []byte
}

func NewServer(
	listener net.Listener,
	signer signer.Signer,
	ctx context.Context,
) *Server {
	return (&Server{
		clients: make(map[client.ClientID]*client.Client),
	}).SetListener(
		listener,
	).SetSigner(
		signer,
	).SetContext(
		ctx,
	)
}

func (s *Server) SetListener(listener net.Listener) *Server {
	if listener == nil {
		file, _ := ioutil.TempFile("", "")
		listener, _ = net.FileListener(file)
	}

	s.listener = listener
	return s
}

func (s *Server) SetSigner(sign signer.Signer) *Server {
	if sign == nil {
		sign = signer.NewClearText()
	}

	s.signer = sign
	return s
}

func (s *Server) SetContext(ctx context.Context) *Server {
	if ctx == nil {
		ctx = context.Background()
	}

	s.ctx = ctx
	return s
}

func (s *Server) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}

		client := s.addClient(conn)
		go func() {
			for !client.Done() {
				encMsg, err := client.GetMsg()
				if err != nil {
					break
				}

				msg, err := s.signer.Decode(encMsg)
				if err != nil {
					break
				}

				msg = append([]byte(fmt.Sprintf("%d: ", client.GetID())), msg...)

				s.BroadcastMsg(msg)
			}

			s.clientsLock.Lock()
			s.removeClient(client)
			s.clientsLock.Unlock()

			s.BroadcastMsg([]byte(fmt.Sprintf("Client %d has disconnected\n", client.GetID())))
		}()

		s.BroadcastMsg([]byte(fmt.Sprintf("Client %d has connected\n", client.GetID())))
	}
}

func (s *Server) Close() (errs []error) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()

	clientCount := len(s.clients)
	errChan := make(chan error, clientCount)
	for _, c := range s.clients {
		go func(client *client.Client) {
			errChan <- client.Close()
		}(c)
	}

	for i := 0; i < clientCount; i++ {
		err := <-errChan
		if err != nil {
			errs = append(errs, err)
		}
	}

	return
}

func (s *Server) BroadcastMsg(msg []byte) error {
	encMsg, err := s.signer.Encode(msg)
	if err != nil {
		return err
	}

	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()

	wg := sync.WaitGroup{}
	for _, c := range s.clients {
		wg.Add(1)
		go func(client *client.Client) {
			err := client.SendMsg(encMsg)
			if err != nil {
				s.removeClient(client)
				wg.Done()
				s.BroadcastMsg([]byte(fmt.Sprintf("Client %d has disconnected\n", client.GetID())))
				return
			}

			wg.Done()
		}(c)
	}

	wg.Wait()
	return nil
}

func (s *Server) addClient(conn net.Conn) *client.Client {
	s.clientsLock.Lock()

	client := client.NewClient(conn, s.ctx)
	s.clients[client.GetID()] = client

	s.clientsLock.Unlock()

	return client
}

func (s *Server) removeClient(client *client.Client) {
	delete(s.clients, client.GetID())
	client.Close()
}
