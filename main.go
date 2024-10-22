package main

import (
	"DiscordRoleSync/src/boot"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	config, err := boot.LoadConfig()
	if err != nil {
		log.Fatalf("loadConfig: %v", err)
	}

	logFile, err := boot.Init(config)
	defer func() {
		if logFile != nil {
			err := logFile.Close()
			if err != nil {
				log.Printf("ERROR closeLog: %v\n", err)
			}
		}
	}()
	if err != nil {
		log.Printf("ERROR inits: %v", err)
		return
	}

	stopBot := make(chan os.Signal, 1)
	signal.Notify(stopBot, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stopBot

	log.Println("Shutting down the bot...")
}
