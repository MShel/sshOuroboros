package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Mshel/ouroboros/internal/game"
	"github.com/Mshel/ouroboros/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

const (
	host string = "0.0.0.0"
	port string = "6996"

	maxConnectionsPerIP = 2
)

var (
	ipCounter = make(map[string]int)
	ipMutex   sync.Mutex
)

func getIP(s ssh.Session) string {
	if addr, ok := s.RemoteAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return s.RemoteAddr().String()
}

func incrementIP(ip string) {
	ipMutex.Lock()
	defer ipMutex.Unlock()
	ipCounter[ip]++
}

func decrementIP(ip string) {
	ipMutex.Lock()
	defer ipMutex.Unlock()
	ipCounter[ip]--
	if ipCounter[ip] <= 0 {
		delete(ipCounter, ip)
	}
}

func getCount(ip string) int {
	ipMutex.Lock()
	defer ipMutex.Unlock()
	return ipCounter[ip]
}

func connectionLimiterMiddleware(next ssh.Handler) ssh.Handler {
	return func(s ssh.Session) {
		log.Debug("ConnectionLimiterMiddleware running for new authenticated session.")

		ip := getIP(s)

		currentCount := getCount(ip)

		if currentCount >= maxConnectionsPerIP {
			log.Warn("Connection denied: IP limit exceeded", "ip", ip, "attempted_count", currentCount+1, "current_limit", maxConnectionsPerIP)
			errorMessage := fmt.Sprintf("Too many active connections from your IP (%d/%d). Please try again later.\r\n", currentCount+1, maxConnectionsPerIP)
			s.Write([]byte(errorMessage))
			s.Close()
			return
		}

		incrementIP(ip)

		log.Info("Connection accepted", "ip", ip, "current_count", getCount(ip), "limit", maxConnectionsPerIP)
		next(s)
		decrementIP(ip)
		log.Info("Connection closed and counter decremented", "ip", ip, "count_after", getCount(ip))
	}
}

func main() {
	log.SetLevel(log.DebugLevel)

	sshPKeyPath := os.Getenv("OUROBOROS_PRIVATE_KEY_PATH")

	sshServer, serverCreateErr := wish.NewServer(
		wish.WithAddress(host+":"+port),
		wish.WithHostKeyPath(sshPKeyPath),
		wish.WithMiddleware(
			bubbletea.Middleware(viewHandler),
			logging.Middleware(),
			activeterm.Middleware(),
			connectionLimiterMiddleware,
		),
	)

	if serverCreateErr != nil {
		log.Error("Failed to start ssh server", "error", serverCreateErr)
	}
	serverDoneChannel := make(chan os.Signal, 1)
	// Captturing system signal to kill server
	signal.Notify(serverDoneChannel, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err := sshServer.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			serverDoneChannel <- nil
		}
	}()

	<-serverDoneChannel

	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := sshServer.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}

func viewHandler(sshSession ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := sshSession.Pty()
	gameManager := game.GetNewGameManager()
	go gameManager.StartGameLoop()
	controllerModel := ui.NewControllerModel(gameManager, sshSession, pty.Window.Width, pty.Window.Height)

	return controllerModel, []tea.ProgramOption{tea.WithAltScreen()}
}
