package main

import (
	"fmt"
	"time"

	"github.com/rossmcdonald/telegram_hook"

	log "github.com/sirupsen/logrus"
)

//Log is logger for all programm
var Log *log.Logger

//initLogger take from Config parameters for logger and init logger.
//If we use syslog, we will call initSyslogger.
func initLogger() (*log.Logger, error) {
	hook, err := telegram_hook.NewTelegramHook(
		Config.Title,
		Config.TelegramBot.Token,
		fmt.Sprintf("%d", Config.TelegramBot.ChatIdDebug),
		telegram_hook.WithAsync(false),
		telegram_hook.WithTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatalf("Encountered error when creating Telegram hook: %s", err)
	}

	logger := log.New()
	logger.Hooks.Add(hook)

	return logger, nil
}
