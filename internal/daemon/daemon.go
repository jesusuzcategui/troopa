// Package daemon is the long-running process that reconciles runs and
// advances plans while the TUI is closed.
package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/notify"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// Interval is how often the daemon looks at running agents.
var Interval = 3 * time.Second

// Run holds the pid file, logs to daemon.log and ticks until ctx is done.
func Run(ctx context.Context) error {
	dataDir, configPath, dbPath, err := config.Paths()
	if err != nil {
		return err
	}
	unlock, err := Acquire(dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	logFile, err := os.OpenFile(LogPath(dataDir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags)

	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	i18n.Apply(cfg.Language)
	eng := engine.New(s, cfg, configPath)
	logger.Println("started")
	if tg := cfg.Telegram; tg.Actions && tg.Token != "" && tg.ChatID != "" {
		go notify.NewBot(tg.Token, tg.ChatID, eng, dataDir, logger).Run(ctx)
	}
	defer logger.Println("stopped")

	if err := Tick(eng, logger); err != nil {
		logger.Printf("tick: %v", err)
	}
	t := time.NewTicker(Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := Tick(eng, logger); err != nil {
				logger.Printf("tick: %v", err)
			}
		}
	}
}

// Tick reconciles running agents and advances plans with auto-advance.
func Tick(e *engine.Engine, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	rec, err := e.Reconcile()
	if err != nil {
		return err
	}
	logger.Printf("watching %d running agents", len(rec.Alive))
	plans, err := e.Store.ListPlans(0)
	if err != nil {
		return err
	}
	busy := i18n.T("An agent is already running")
	for _, p := range plans {
		if p.State != store.PlanRunning || !(p.Auto || p.AutoApprove) {
			continue
		}
		res, err := e.AdvancePlan(p.ID)
		if err != nil {
			if err.Error() == busy {
				continue
			}
			logger.Printf("plan %d: %v", p.ID, err)
			continue
		}
		if res.Watch {
			logger.Printf("plan %d launched run %d", p.ID, res.Run.ID)
		}
	}
	return nil
}

// Status writes whether the daemon is up and which runs it would watch.
func Status(w io.Writer) error {
	dataDir, _, dbPath, err := config.Paths()
	if err != nil {
		return err
	}
	pid, ok := Running(dataDir)
	if !ok {
		fmt.Fprintln(w, i18n.T("Troopa daemon is not running"))
		return nil
	}
	fmt.Fprintln(w, i18n.T("Troopa daemon is running (pid %d)", pid))
	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	runs, err := s.ListRuns(store.RunRunning)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		fmt.Fprintln(w, i18n.T("No running agents to watch"))
		return nil
	}
	fmt.Fprintln(w, i18n.T("Watching:"))
	for _, r := range runs {
		title := ""
		if t, err := s.GetTask(r.TaskID); err == nil {
			title = t.Title
		}
		fmt.Fprintln(w, i18n.T("run %d · %s · %s", r.ID, title, r.Agent))
	}
	return nil
}
