package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	relay "wake-on-wan-relay/lib"
)

func (s *ServerApp) StatusHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received /status call")

	context := relay.GetContext()
	portOpen, err := CheckSSH(context.InternalAddr, 22, 1*time.Second)

	if portOpen && err == nil {
		w.Write([]byte("Server is up and SSH is enabled"))
		return
	}

	if !portOpen {
		http.Error(w, "SSH Server seems to be down", http.StatusExpectationFailed)
		return
	}

	if portOpen && err != nil {
		http.Error(w, "Server is up but port seems to be occupied by another process", http.StatusLocked)
		return
	}

	http.Error(w, "Unknown Server Error. Please contact your administrator", http.StatusInternalServerError)

}

func CheckSSH(host string, port int, timeout time.Duration) (portOpen bool, err error) {
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return false, fmt.Errorf("SSH Connection Failed: %w", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(timeout))
	reader := bufio.NewReader(conn)

	banner, err := reader.ReadString('\n')
	if err != nil {
		return true, fmt.Errorf("Port Open, Failed to Read Banner: %w", err)
	}

	banner = strings.TrimSpace(banner)
	if strings.HasPrefix(banner, "SSH-") {
		return true, nil
	}

	return true, fmt.Errorf("Port Open, Not SSH")
}
