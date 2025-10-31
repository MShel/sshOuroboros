package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/Mshel/sshnake/internal/game"
	"github.com/Mshel/sshnake/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

func startPprofServer() {
	// Run pprof server on port 6060. This server is distinct from your main game server.
	fmt.Println(http.ListenAndServe("localhost:6060", nil))
}

const (
	host string = "0.0.0.0"
	port string = "6996"
)

func main() {
	go startPprofServer()
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	sshPKeyPath := os.Getenv("OUROBOROS_PRIVATE_KEY_PATH")

	sshServer, serverCreateErr := wish.NewServer(
		wish.WithAddress(host+":"+port),
		wish.WithHostKeyPath(sshPKeyPath),
		wish.WithMiddleware(
			bubbletea.Middleware(viewHandler),
			activeterm.Middleware(),
			logging.Middleware(),
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
