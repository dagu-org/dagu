// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/persis/filesession"
	"github.com/dagu-org/dagu/internal/service/telegram"
	"github.com/spf13/cobra"
)

// Telegram returns the cobra command for the Telegram bot service.
func Telegram() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "telegram [flags]",
			Short: "Start the Telegram bot for AI agent interaction",
			Long: `Launch a Telegram bot that allows interacting with the Dagu AI agent
via Telegram chat messages.

The bot maps each Telegram chat to an agent session. You can send messages
to chat with the AI agent, and the agent can execute tools, run DAGs, and
answer questions about your workflows.

Configuration:
  Set DAGU_TELEGRAM_TOKEN to your Telegram bot token (from @BotFather).
  Optionally set allowed_chat_ids in config to restrict access.

Commands in Telegram:
  /new    - Start a fresh session
  /cancel - Cancel the current session

Example:
  DAGU_TELEGRAM_TOKEN=your-bot-token dagu telegram --dags=/path/to/dags
`,
		}, telegramFlags, runTelegram,
	)
}

var telegramFlags = []commandLineFlag{dagsFlag}

func runTelegram(ctx *Context, _ []string) error {
	signalCtx, stop := signal.NotifyContext(ctx.Context, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serviceCtx := ctx.WithContext(signalCtx)

	if ctx.Config.Telegram.Token == "" {
		return fmt.Errorf("telegram bot token is required: set DAGU_TELEGRAM_TOKEN environment variable or telegram.token in config")
	}

	// Initialize agent stores
	stores := ctx.agentStores()
	if stores.ConfigStore == nil {
		return fmt.Errorf("failed to initialize agent config store")
	}

	// Create session store
	sessStore, err := filesession.New(ctx.Config.Paths.SessionsDir,
		filesession.WithMaxPerUser(ctx.Config.Server.Session.MaxPerUser),
	)
	if err != nil {
		logger.Warn(serviceCtx, "Failed to create session store, persistence disabled", tag.Error(err))
	}

	hooks := agent.NewHooks()

	agentAPI := agent.NewAPI(agent.APIConfig{
		ConfigStore:        stores.ConfigStore,
		ModelStore:         stores.ModelStore,
		SkillStore:         stores.SkillStore,
		SoulStore:          stores.SoulStore,
		WorkingDir:         ctx.Config.Paths.DAGsDir,
		Logger:             slog.Default(),
		SessionStore:       sessStore,
		Hooks:              hooks,
		MemoryStore:        stores.MemoryStore,
		RemoteNodeResolver: stores.RemoteNodeResolver,
		Environment: agent.EnvironmentInfo{
			DAGsDir:        ctx.Config.Paths.DAGsDir,
			DocsDir:        ctx.Config.Paths.DocsDir,
			LogDir:         ctx.Config.Paths.LogDir,
			DataDir:        ctx.Config.Paths.DataDir,
			ConfigFile:     ctx.Config.Paths.ConfigFileUsed,
			WorkingDir:     ctx.Config.Paths.DAGsDir,
			BaseConfigFile: ctx.Config.Paths.BaseConfig,
		},
	})

	agentAPI.StartCleanup(signalCtx)

	bot, err := telegram.New(
		telegram.Config{
			Token:          ctx.Config.Telegram.Token,
			AllowedChatIDs: ctx.Config.Telegram.AllowedChatIDs,
			SafeMode:       ctx.Config.Telegram.SafeMode,
			DAGRunStore:    ctx.DAGRunStore,
		},
		agentAPI,
		slog.Default(),
	)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	logger.Info(serviceCtx, "Starting Telegram bot")

	return bot.Run(signalCtx)
}
