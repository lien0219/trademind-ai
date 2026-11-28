package smtp

import (
	"bufio"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeliverMessageUsesDataAcceptanceEvenIfQuitFails(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dataStatus string
		wantErr    string
	}{
		{"accepted", "250 queued", ""},
		{"rejected", "554 not accepted", "smtp: finalize"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer clientConn.Close()
			defer serverConn.Close()
			deadline := time.Now().Add(3 * time.Second)
			require.NoError(t, clientConn.SetDeadline(deadline))
			require.NoError(t, serverConn.SetDeadline(deadline))
			serverDone := make(chan error, 1)
			go func() {
				defer serverConn.Close()
				serverDone <- serveTestSMTPData(serverConn, tc.dataStatus)
			}()

			client, err := smtp.NewClient(clientConn, "mail.example.test")
			require.NoError(t, err)
			defer client.Close()
			err = deliverMessage(client, "Subject: test\r\n\r\nverification message\r\n")
			require.NoError(t, <-serverDone)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

// Only the DATA exchange is under test. The synthetic transport closes the
// connection as soon as the server acknowledges or rejects the message.
func serveTestSMTPData(conn net.Conn, status string) error {
	if _, err := fmt.Fprint(conn, "220 mail.example.test ESMTP\r\n"); err != nil {
		return err
	}
	r := bufio.NewReader(conn)
	cmd, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if cmd != "DATA\r\n" {
		return fmt.Errorf("expected DATA, received %q", cmd)
	}
	if _, err := fmt.Fprint(conn, "354 continue\r\n"); err != nil {
		return err
	}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(line) == "." {
			_, err := fmt.Fprintf(conn, "%s\r\n", status)
			return err
		}
	}
}
