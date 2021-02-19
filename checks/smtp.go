package checks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
)

type Smtp struct {
	checkBase
	Sender    string
	Receiver  string
	Body      string
	Encrypted bool
}

type unencryptedAuth struct {
	smtp.Auth
}

func (a unencryptedAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	s := *server
	s.TLS = true
	return a.Auth.Start(&s)
}

func (c Smtp) Run(teamName, boxIp string, res chan Result) {
	// Create a dialer
	dialer := net.Dialer{
		Timeout: GlobalTimeout,
	}

	// ***********************************************
	// Set up custom auth for bypassing net/smtp protections
	username, password := getCreds(c.CredLists, teamName, c.Name)
	auth := unencryptedAuth{smtp.PlainAuth("", username, password, boxIp)}
	// ***********************************************

	// The good way to do auth
	// auth := smtp.PlainAuth("", d.Username, d.Password, d.Host)
	// Create TLS config
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}

	// Declare these for the below if block
	var conn net.Conn
	var err error

	if c.Encrypted {
		conn, err = tls.DialWithDialer(&dialer, "tcp", fmt.Sprintf("%s:%s", boxIp, c.Port), &tlsConfig)
	} else {
		conn, err = dialer.DialContext(context.TODO(), "tcp", fmt.Sprintf("%s:%s", boxIp, c.Port))
	}
	if err != nil {
		res <- Result{
			Error: "connection to server failed",
			Debug: err.Error(),
		}
		return
	}
	defer conn.Close()

	// Create smtp client
	sconn, err := smtp.NewClient(conn, boxIp)
	if err != nil {
		res <- Result{
			Error: "smtp client creation failed",
			Debug: err.Error(),
		}
		return
	}
	defer sconn.Quit()

	// Login
	err = sconn.Auth(auth)
	if err != nil {
		res <- Result{
			Error: "login failed",
			Debug: err.Error(),
		}
		return
	}

	// Set the sender
	err = sconn.Mail(c.Sender)
	if err != nil {
		res <- Result{
			Error: "setting sender failed",
			Debug: err.Error(),
		}
		return
	}

	// Set the reciver
	err = sconn.Rcpt(c.Receiver)
	if err != nil {
		res <- Result{
			Error: "setting receiver failed",
			Debug: err.Error(),
		}
		return
	}

	// Create email writer
	wc, err := sconn.Data()
	if err != nil {
		res <- Result{
			Error: "creating email writer failed",
			Debug: err.Error(),
		}
		return
	}
	defer wc.Close()

	// Write the body
	_, err = fmt.Fprintf(wc, c.Body)
	if err != nil {
		res <- Result{
			Error: "writing body failed",
			Debug: err.Error(),
		}
		return
	}

	res <- Result{
		Status: true,
		Debug:  "successfully wrote '" + c.Body + "' to " + c.Receiver + " from " + c.Sender,
	}
	return
}
