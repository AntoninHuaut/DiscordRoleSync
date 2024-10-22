package domain

const (
	ConfigFileName = "config.yaml"
	LogFileName    = "storage/log.txt"
)

type Config struct {
	Discord DiscordConfig `yaml:"discord"`
}

type DiscordConfig struct {
	Token string `yaml:"token"`
}
