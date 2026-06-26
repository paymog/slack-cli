// Package runtime bootstraps the reused provider stack for CLI commands:
// a quiet logger and an ApiProvider built from resolved credentials, plus
// on-disk cache loading for read commands.
package runtime

import (
	"context"
	"os"

	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/pkg/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger builds a stderr zap logger. By default only Fatal is emitted (so a
// normal run prints just the command's result on stdout, while provider auth
// failures still surface). --verbose raises it to Debug for full handler logs.
func Logger(verbose bool) *zap.Logger {
	level := zapcore.FatalLevel
	if verbose {
		level = zapcore.DebugLevel
	}
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encCfg),
		zapcore.Lock(os.Stderr),
		level,
	)
	return zap.New(core)
}

// Provider builds an ApiProvider from resolved config. It requires a usable
// credential set and writes tokens into the SLACK_MCP_* env so the reused
// provider.New picks them up.
func Provider(cfg *config.Config) (*provider.ApiProvider, *zap.Logger, error) {
	if err := cfg.RequireAuth(); err != nil {
		return nil, nil, err
	}
	cfg.ApplyToEnv()
	logger := Logger(cfg.Verbose)
	return provider.New("stdio", logger), logger, nil
}

// PrepareRead builds a provider and loads the on-disk user/channel caches so
// that #channel-name and @username lookups resolve. With --no-cache it marks
// the provider ready without loading (only channel/user IDs will resolve).
func PrepareRead(ctx context.Context, cfg *config.Config) (*provider.ApiProvider, *zap.Logger, error) {
	p, logger, err := Provider(cfg)
	if err != nil {
		return nil, nil, err
	}
	if cfg.NoCache {
		p.SkipCache()
		return p, logger, nil
	}
	_ = p.RefreshUsers(ctx)
	_ = p.RefreshChannels(ctx)
	return p, logger, nil
}
