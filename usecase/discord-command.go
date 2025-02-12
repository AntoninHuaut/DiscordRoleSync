package usecase

import (
	domain2 "DiscordRoleSync/domain"
	"github.com/bwmarrin/discordgo"
	"slices"
)

type DiscordCommand interface {
	InitCommands() error
	GetSession() *discordgo.Session
}

func NewDiscordCommand(config *domain2.Config, dcSession *discordgo.Session) DiscordCommand {
	return &discordCommand{
		config:    config,
		dcSession: dcSession,
		autoRole:  NewDiscordCommandAutoRole(config),
		roleSync:  NewDiscordCommandRoleSync(config),
	}
}

type discordCommand struct {
	config    *domain2.Config
	dcSession *discordgo.Session
	autoRole  DiscordCommandAutoRole
	roleSync  DiscordCommandRoleSync
}

func (d *discordCommand) InitCommands() error {
	d.dcSession.AddHandler(d.commandHandler)

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
			if slices.Contains([]string{domain2.AutoRoleCommandName, domain2.RoleSyncCommandName}, application.Name) {
				err := d.dcSession.ApplicationCommandDelete(d.dcSession.State.User.ID, guild.ID, application.ID)
				if err != nil {
					return err
				}
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
			Name:                     domain2.RoleSyncCommandName,
			Type:                     discordgo.ChatApplicationCommand,
			Description:              "Sync two roles",
			DefaultMemberPermissions: &adminPerm,
		}

		roleSyncCommand.Options = []*discordgo.ApplicationCommandOption{{
			Name:         domain2.RoleSyncCommandOptionOrigin,
			Description:  "It's the role that will be used to retrieve the permissions",
			Type:         discordgo.ApplicationCommandOptionString,
			Required:     true,
			Autocomplete: true,
		}, {
			Name:         domain2.RoleSyncCommandOptionTarget,
			Description:  "It's the role that will receive the permissions",
			Type:         discordgo.ApplicationCommandOptionString,
			Required:     true,
			Autocomplete: true,
		}}

		if _, err := d.dcSession.ApplicationCommandCreate(d.dcSession.State.User.ID, guild.ID, &roleSyncCommand); err != nil {
			return err
		}

		if autoRoles, ok := d.config.Discord.AutoRoles[guild.ID]; ok {
			autoRolesCommand := discordgo.ApplicationCommand{
				Name:        domain2.AutoRoleCommandName,
				Type:        discordgo.ChatApplicationCommand,
				Description: "Get auto roles",
			}

			var choices []*discordgo.ApplicationCommandOptionChoice

			for _, autoRole := range autoRoles {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  autoRole.Name,
					Value: autoRole.Id,
				})
			}

			autoRolesCommand.Options = []*discordgo.ApplicationCommandOption{{
				Name:        domain2.AutoRoleCommandOptionRole,
				Description: "Role to get",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
				Choices:     choices,
			}}

			if _, err := d.dcSession.ApplicationCommandCreate(d.dcSession.State.User.ID, guild.ID, &autoRolesCommand); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *discordCommand) commandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cmdName := i.ApplicationCommandData().Name
	if cmdName == domain2.RoleSyncCommandName {
		d.roleSync.RoleSyncCommandHandler(s, i)
	} else if cmdName == domain2.AutoRoleCommandName {
		d.autoRole.AutoRoleCommandHandler(s, i)
	}
}
