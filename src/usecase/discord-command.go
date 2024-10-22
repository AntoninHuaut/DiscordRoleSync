package usecase

import (
	"DiscordRoleSync/src/domain"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"slices"
)

type DiscordCommand interface {
	InitCommands() error
	GetSession() *discordgo.Session
}

func NewDiscordCommand(config *domain.Config, dcSession *discordgo.Session) DiscordCommand {
	return &discordCommand{
		config:    config,
		dcSession: dcSession,
	}
}

type discordCommand struct {
	config    *domain.Config
	dcSession *discordgo.Session
}

func (d *discordCommand) InitCommands() error {
	d.dcSession.AddHandler(d.roleSyncCommandHandler)

	err := d.unregisterCommands()
	if err != nil {
		return err
	}

	err = d.registerCommands()
	if err != nil {
		return err
	}

	return nil
}

func (d *discordCommand) GetSession() *discordgo.Session {
	return d.dcSession
}

func (d *discordCommand) unregisterCommands() error {
	guilds, err := d.dcSession.UserGuilds(200, "", "", false)
	if err != nil {
		return err
	}

	for _, guild := range guilds {
		applications, err := d.dcSession.ApplicationCommands(d.dcSession.State.User.ID, guild.ID)
		if err != nil {
			return err
		}

		for _, application := range applications {
			if application.Name != domain.RoleSyncCommandName {
				continue
			}

			err := d.dcSession.ApplicationCommandDelete(d.dcSession.State.User.ID, guild.ID, application.ID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *discordCommand) registerCommands() error {
	guilds, err := d.dcSession.UserGuilds(200, "", "", false)
	if err != nil {
		return err
	}

	for _, guild := range guilds {
		adminPerm := int64(discordgo.PermissionAdministrator)
		roleSyncCommand := discordgo.ApplicationCommand{
			Name:                     domain.RoleSyncCommandName,
			Type:                     discordgo.ChatApplicationCommand,
			Description:              "Sync two roles",
			DefaultMemberPermissions: &adminPerm,
		}

		roleSyncCommand.Options = []*discordgo.ApplicationCommandOption{{
			Name:         domain.RoleSyncCommandOptionOrigin,
			Description:  "It's the role that will be used to retrieve the permissions",
			Type:         discordgo.ApplicationCommandOptionString,
			Required:     true,
			Autocomplete: true,
		}, {
			Name:         domain.RoleSyncCommandOptionTarget,
			Description:  "It's the role that will receive the permissions",
			Type:         discordgo.ApplicationCommandOptionString,
			Required:     true,
			Autocomplete: true,
		}}

		if _, err := d.dcSession.ApplicationCommandCreate(d.dcSession.State.User.ID, guild.ID, &roleSyncCommand); err != nil {
			return err
		}
	}

	return nil
}

func (d *discordCommand) roleSyncCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
		roleSyncCommandAutoComplete(s, i)
		return
	} else if i.Type != discordgo.InteractionApplicationCommand || i.ApplicationCommandData().Name != domain.RoleSyncCommandName {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) < 2 {
		return // Must never happen, option is required
	}

	originRoleId := options[0].StringValue()
	targetRoleId := options[1].StringValue()

	isGuildPermissionsUpdated, nbUpdatedChannels, err := syncRolePermissions(s, i.GuildID, originRoleId, targetRoleId)
	responseMessage := ""
	if err != nil {
		log.Printf("ERROR syncing roles: %v", err)
		responseMessage = fmt.Sprintf("An error occurred while syncing roles: %v", err)
	} else {
		responseMessage = "Roles synced successfully"
		if isGuildPermissionsUpdated {
			responseMessage += ", guild permissions updated"
		}
		if nbUpdatedChannels > 0 {
			responseMessage += fmt.Sprintf(", %d channels updated", nbUpdatedChannels)
		}
		if !isGuildPermissionsUpdated && nbUpdatedChannels == 0 {
			responseMessage += ", but nothing to update"
		}
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: responseMessage},
	}); err != nil {
		log.Printf("ERROR responding to command: %v", err)
		return
	}
}

func roleSyncCommandAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var focusedOption *discordgo.ApplicationCommandInteractionDataOption

	for _, option := range i.ApplicationCommandData().Options {
		if option.Focused {
			focusedOption = option
			break
		}
	}

	if focusedOption == nil {
		return
	}

	if slices.Contains([]string{domain.RoleSyncCommandOptionOrigin, domain.RoleSyncCommandOptionTarget}, focusedOption.Name) {
		userInput := focusedOption.StringValue()

		guildRoles, err := s.GuildRoles(i.GuildID)
		if err != nil {
			log.Printf("ERROR fetching roles: %v", err)
			return
		}

		var choices []*discordgo.ApplicationCommandOptionChoice
		for _, role := range guildRoles {
			if len(userInput) == 0 || (len(role.Name) >= len(userInput) && role.Name[:len(userInput)] == userInput) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  role.Name,
					Value: role.ID,
				})
			}
		}

		// Discord limits the number of choices to 25
		if len(choices) > 25 {
			choices = choices[:25]
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		})
		if err != nil {
			log.Printf("ERROR responding to autocomplete: %v", err)
		}
	}
}

func syncRolePermissions(s *discordgo.Session, guildID, originRoleId, targetRoleId string) (bool, int, error) {
	originRole, originRoleErr := s.State.Role(guildID, originRoleId)
	if originRoleErr != nil {
		return false, 0, originRoleErr
	}
	targetRole, targetRoleErr := s.State.Role(guildID, targetRoleId)
	if targetRoleErr != nil {
		return false, 0, targetRoleErr
	}

	isGuildPermissionsDifferent := originRole.Permissions != targetRole.Permissions
	if isGuildPermissionsDifferent {
		roleParams := &discordgo.RoleParams{
			Hoist:       &originRole.Hoist,
			Mentionable: &originRole.Mentionable,
			Permissions: &originRole.Permissions,
		}

		if _, guildEditErr := s.GuildRoleEdit(guildID, targetRoleId, roleParams); guildEditErr != nil {
			return false, 0, guildEditErr
		}
	}

	channels, guildChannelsErr := s.GuildChannels(guildID)
	if guildChannelsErr != nil {
		return isGuildPermissionsDifferent, 0, guildChannelsErr
	}

	nbUpdatedChannels := 0
	for _, channel := range channels {
		var originRolePerms *discordgo.PermissionOverwrite
		var targetRolePerms *discordgo.PermissionOverwrite
		for _, permissionOverwrite := range channel.PermissionOverwrites {
			if permissionOverwrite.Type != discordgo.PermissionOverwriteTypeRole {
				continue
			}

			if permissionOverwrite.ID == originRoleId {
				originRolePerms = permissionOverwrite
			} else if permissionOverwrite.ID == targetRoleId {
				targetRolePerms = permissionOverwrite
			}
		}

		// If no permissions are set for both roles, skip
		if originRolePerms == nil && targetRolePerms == nil {
			continue
		}

		// If permissions are the same, skip
		if originRolePerms != nil && targetRolePerms != nil && originRolePerms.Allow == targetRolePerms.Allow && originRolePerms.Deny == targetRolePerms.Deny {
			continue
		}

		if originRolePerms == nil {
			// Delete permissions for target role
			if delErr := s.ChannelPermissionDelete(channel.ID, targetRoleId); delErr != nil {
				return isGuildPermissionsDifferent, nbUpdatedChannels, delErr
			}
		} else {
			// Create or update permissions for target role
			if createErr := s.ChannelPermissionSet(channel.ID, targetRoleId, discordgo.PermissionOverwriteTypeRole, originRolePerms.Allow, originRolePerms.Deny); createErr != nil {
				return isGuildPermissionsDifferent, nbUpdatedChannels, createErr
			}
		}

		nbUpdatedChannels++
	}

	return isGuildPermissionsDifferent, nbUpdatedChannels, nil
}
