package usecase

import (
	domain2 "DiscordRoleSync/domain"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"slices"
)

type DiscordCommandRoleSync interface {
	RoleSyncCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate)
}

func NewDiscordCommandRoleSync(config *domain2.Config) DiscordCommandRoleSync {
	return &discordCommandRoleSync{
		config: config,
	}
}

type discordCommandRoleSync struct {
	config *domain2.Config
}

func (d *discordCommandRoleSync) RoleSyncCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
		roleSyncCommandAutoComplete(s, i)
		return
	} else if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) < 2 {
		return // Must never happen, option is required
	}

	originRoleId := options[0].StringValue()
	targetRoleId := options[1].StringValue()

	isGuildPermissionsUpdated, nbUpdatedChannels, err := syncRolePermissions(s, i.GuildID, originRoleId, targetRoleId)
	responseMessage := "Roles synced successfully"
	if err != nil {
		log.Printf("ERROR syncing roles: %v", err)
		responseMessage = fmt.Sprintf("An error occurred while syncing roles: %v", err)
	} else {
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

	if slices.Contains([]string{domain2.RoleSyncCommandOptionOrigin, domain2.RoleSyncCommandOptionTarget}, focusedOption.Name) {
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
