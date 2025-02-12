package domain

const (
	ConfigFileName = "config.yaml"
	LogFileName    = "storage/log.txt"
)

type Config struct {
	Discord DiscordConfig `yaml:"discord"`
}

type DiscordConfig struct {
	Token     string                 `yaml:"token"`
	AutoRoles map[string][]GuildRole `yaml:"auto_roles"`
}

type GuildRole struct {
	Id        string             `yaml:"id"`
	Name      string             `yaml:"name"`
	Condition GuildRoleCondition `yaml:"condition"`
}

type GuildRoleCondition struct {
	AgeOnServer string `yaml:"age_on_server"`
}
