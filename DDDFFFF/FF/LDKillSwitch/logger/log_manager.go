package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"featuresgflags/LDKillSwitch/service"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

// internal logger that always writes at ERROR level to avoid being filtered
var internalLog = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

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

	// Wait a moment for the main goroutine to install the dynamic handler.
	// Alternatively, call refreshLevel synchronously after SetDefault in main.
	time.Sleep(100 * time.Millisecond)
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

	var levelsSlice []string

	// Robust parsing: handle various possible types from LaunchDarkly
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				levelsSlice = append(levelsSlice, str)
			}
		}
	case []string:
		levelsSlice = v
	case json.RawMessage:
		// Try to unmarshal as []string first
		var strSlice []string
		if err := json.Unmarshal(v, &strSlice); err == nil {
			levelsSlice = strSlice
		} else {
			// Fallback to []interface{}
			var ifaceSlice []interface{}
			if err := json.Unmarshal(v, &ifaceSlice); err == nil {
				for _, item := range ifaceSlice {
					if str, ok := item.(string); ok {
						levelsSlice = append(levelsSlice, str)
					}
				}
			} else {
				internalLog.Error("Log level flag is json.RawMessage but cannot unmarshal", "value", string(v))
				return
			}
		}
	case string:
		// Sometimes the flag might be returned as a JSON string
		var tmp []string
		if err := json.Unmarshal([]byte(v), &tmp); err == nil {
			levelsSlice = tmp
		} else {
			internalLog.Error("Log level flag is a string but not a valid JSON array", "value", v)
			return
		}
	default:
		internalLog.Error("Log level flag returned unexpected type", "type", fmt.Sprintf("%T", raw), "value", raw)
		return
	}

	if len(levelsSlice) == 0 {
		internalLog.Warn("Log level flag returned empty slice", "value", raw)
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
			internalLog.Warn("Unknown log level in list", "level", str)
			continue
		}
		if level < minLevel {
			minLevel = level
		}
	}

	old := m.level.Swap(minLevel).(slog.Level)
	if old != minLevel {
		// Use internal logger (always ERROR) so this message is never filtered
		internalLog.Info("Log level changed dynamically", "from", old.String(), "to", minLevel.String(), "enabled_levels", levelsSlice)
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
