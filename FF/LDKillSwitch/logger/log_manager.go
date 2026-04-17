package logging

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"featuresgflags/LDKillSwitch/service"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

type LogLevelManager struct {
	flagSvc      *service.FeatureFlagService
	flagKey      string
	ctx          ldcontext.Context
	pollInterval time.Duration
	level        atomic.Value
	stopCh       chan struct{}
}

func NewLogLevelManager(
	flagSvc *service.FeatureFlagService,
	flagKey string,
	ctx ldcontext.Context,
	pollInterval time.Duration,
) *LogLevelManager {
	mgr := &LogLevelManager{
		flagSvc:      flagSvc,
		flagKey:      flagKey,
		ctx:          ctx,
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}
	mgr.level.Store(slog.LevelInfo)
	return mgr
}

func (m *LogLevelManager) Start() {
	go m.pollLoop()
}

func (m *LogLevelManager) Stop() {
	close(m.stopCh)
}

func (m *LogLevelManager) Level() slog.Level {
	return m.level.Load().(slog.Level)
}

func (m *LogLevelManager) pollLoop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	m.refreshLevel()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.refreshLevel()
		}
	}
}
func (m *LogLevelManager) refreshLevel() {
	defaultValue := []interface{}{"INFO"}
	raw := m.flagSvc.GetJSONFlag(m.flagKey, m.ctx, defaultValue)
	slog.Info("refreshLevel raw type", "type", fmt.Sprintf("%T", raw), "value", raw)
	// Handle both []interface{} and []string
	var levelsSlice []string
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				levelsSlice = append(levelsSlice, str)
			}
		}
	case []string:
		levelsSlice = v
	default:
		slog.Warn("Log level flag did not return a slice", "value", raw)
		return
	}

	if len(levelsSlice) == 0 {
		slog.Warn("Log level flag returned empty slice", "value", raw)
		return
	}

	var minLevel slog.Level = slog.LevelError
	for _, str := range levelsSlice {
		var level slog.Level
		switch str {
		case "DEBUG":
			level = slog.LevelDebug
		case "INFO":
			level = slog.LevelInfo
		case "WARN":
			level = slog.LevelWarn
		case "ERROR":
			level = slog.LevelError
		default:
			slog.Warn("Unknown log level in list", "level", str)
			continue
		}
		if level < minLevel {
			minLevel = level
		}
	}

	old := m.level.Swap(minLevel).(slog.Level)
	if old != minLevel {
		slog.Info("Log level changed dynamically", "from", old, "to", minLevel, "enabled_levels", levelsSlice)
	}
}
func (m *LogLevelManager) SetLogLevelHandler(baseHandler slog.Handler) slog.Handler {
	return &dynamicLevelHandler{
		Handler: baseHandler,
		levelFn: m.Level,
	}
}

type dynamicLevelHandler struct {
	slog.Handler
	levelFn func() slog.Level
}

func (h *dynamicLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.levelFn()
}
