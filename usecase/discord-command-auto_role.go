package usecase

import (
	"DiscordRoleSync/domain"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"time"
)

type DiscordCommandAutoRole interface {
	AutoRoleCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate)
}

func NewDiscordCommandAutoRole(config *domain.Config) DiscordCommandAutoRole {
	return &discordCommandAutoRole{
		config: config,
	}
}

type discordCommandAutoRole struct {
	config *domain.Config
}

func (d *discordCommandAutoRole) AutoRoleCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return // Must never happen, option is required
	}

	roleId := options[0].StringValue()
	userId := i.Member.User.ID

	responseMessage := "Role added successfully"
	guildRole, err := getGuildRole(d.config.Discord.AutoRoles, i.GuildID, roleId)
	if err != nil {
		responseMessage = err.Error()
	} else {
		if giveRoleErr := giveUserRoleIfConditionMet(s, guildRole, i.GuildID, userId); giveRoleErr != nil {
			responseMessage = giveRoleErr.Error()
		}
	}

	respErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: responseMessage},
	})
	if respErr != nil {
		log.Printf("ERROR responding to command: %v", respErr)
		return
	}
}

func getGuildRoles(autoRoles map[string][]domain.GuildRole, guildId string) ([]domain.GuildRole, error) {
	guildRoles, ok := autoRoles[guildId]
	if !ok {
		return nil, fmt.Errorf("guild not found in config: %v", guildId)
	}
	return guildRoles, nil
}

func getGuildRole(autoRoles map[string][]domain.GuildRole, guildId string, roleId string) (*domain.GuildRole, error) {
	guildRoles, err := getGuildRoles(autoRoles, guildId)
	if err != nil {
		return nil, err
	}

	for _, role := range guildRoles {
		if role.Id == roleId {
			return &role, nil
		}
	}
	return nil, fmt.Errorf("role not found in config: %v", roleId)
}

func giveUserRoleIfConditionMet(s *discordgo.Session, guildRole *domain.GuildRole, guildId string, userId string) error {
	member, err := s.GuildMember(guildId, userId)
	if err != nil {
		return errors.New("cannot fetch member")
	}

	for _, role := range member.Roles {
		if role == guildRole.Id {
			return errors.New("user already has the role")
		}
	}

	if ageOnServErr := checkConditionAgeOnServ(guildRole, member); ageOnServErr != nil {
		return ageOnServErr
	}

	if addErr := s.GuildMemberRoleAdd(guildId, userId, guildRole.Id); addErr != nil {
		return errors.New("cannot add role to user")
	}

	return nil
}

func checkConditionAgeOnServ(guildRole *domain.GuildRole, member *discordgo.Member) error {
	duration, err := time.ParseDuration(guildRole.Condition.AgeOnServer)
	if err != nil {
		return errors.New("cannot parse duration")
	}

	if guildRole.Condition.AgeOnServer != "" {
		if member.JoinedAt.Add(duration).After(time.Now()) {
			return errors.New("user does not meet the required age on server")
		}
	}

	return nil
}
