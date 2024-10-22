package boot

import (
	"DiscordRoleSync/src/domain"
	"DiscordRoleSync/src/usecase"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/yaml.v3"
)

func LoadConfig() (*domain.Config, error) {
	file, err := os.Open(domain.ConfigFileName)
	if err != nil {
		return nil, err
	}

	all, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config domain.Config
	if err = yaml.Unmarshal(all, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func Init(config *domain.Config) (*os.File, error) {
	logFile, err := initLog()
	if err != nil {
		return nil, err
	}

	err = initDiscord(config)
	if err != nil {
		return logFile, err
	}

	return logFile, nil
}

func initLog() (*os.File, error) {
	err := os.MkdirAll(filepath.Join(".", filepath.Dir(domain.LogFileName)), os.ModePerm)
	if err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(domain.LogFileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	return logFile, nil
}

func initDiscord(config *domain.Config) error {
	dcSession, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		return err
	}

	dcCommand := usecase.NewDiscordCommand(config, dcSession)

	dcSession.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})

	err = dcSession.Open()
	if err != nil {
		return err
	}

	err = dcCommand.InitCommands()
	if err != nil {
		return err
	}

	return nil
}
