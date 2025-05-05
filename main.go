package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

type Config struct {
	BotToken      string
	ApplicationId string
	GuildId       string
	TargetUserId  string
	MuteRoleId    string
}

func newConfig() (Config, error) {
	config := Config{}
	var found = false

	config.BotToken, found = os.LookupEnv("BOT_TOKEN")
	if !found {
		return config, fmt.Errorf("BOT_TOKEN not found")
	}

	config.ApplicationId, found = os.LookupEnv("APPLICATION_ID")
	if !found {
		return config, fmt.Errorf("APPLICATION_ID not found")
	}

	config.GuildId, found = os.LookupEnv("GUILD_ID")
	if !found {
		config.GuildId = ""
	}

	config.TargetUserId, found = os.LookupEnv("TARGET_USER_ID")
	if !found {
		return config, fmt.Errorf("TARGET_USER_ID not found")
	}

	config.MuteRoleId, found = os.LookupEnv("MUTE_ROLE")
	if !found {
		return config, fmt.Errorf("MUTE_ROLE not found")
	}

	return config, nil
}

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("failed to load env variables from file", "err", err)
	}

	config, err := newConfig()
	if err != nil {
		slog.Error("failed to load config from env", "err", err)
		os.Exit(1)
	}

	bot := NewBot(config)

	if err := bot.Run(); err != nil {
		log.Panicln(err)
	}
}

type Bot struct {
	config Config
}

func NewBot(config Config) *Bot {
	return &Bot{
		config: config,
	}
}

func (bot *Bot) Run() error {
	slog.Info("Starting bot", "token", bot.config)
	discord, err := discordgo.New("Bot " + bot.config.BotToken)
	if err != nil {
		return err
	}

	defer discord.Close()

	discord.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		slog.Info("Logged in", slog.String("username", s.State.User.Username), slog.String("discriminator", s.State.User.Discriminator))
	})

	if err := discord.Open(); err != nil {
		return err
	}

	registeredCommands := []*discordgo.ApplicationCommand{
		&discordgo.ApplicationCommand{
			Name:        "mute",
			Description: "Mute smallcap",
		},
	}

	if bot.config.GuildId != "" {
		slog.Info("registering commands in guild", slog.String("guild_id", bot.config.GuildId))
	}

	_, err = discord.ApplicationCommandBulkOverwrite(bot.config.ApplicationId, bot.config.GuildId, registeredCommands)
	if err != nil {
		return err
	}

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if err := bot.handle(s, i); err != nil {
			slog.Error("failed to handle interaction", "err", err)
		}
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	return nil
}

func (bot *Bot) handle(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	userId := bot.config.ApplicationId
	member, err := s.GuildMember(i.GuildID, userId)
	if err != nil {
		return err
	}

	mute := bot.config.MuteRoleId

	if !slices.Contains(member.Roles, mute) {
		if err := s.GuildMemberRoleAdd(i.GuildID, userId, mute); err != nil {
			return err
		}

		now := time.Now()
		end := now.Add(15 * time.Second)

		s.GuildMemberMute(i.GuildID, userId, true)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("<t:%d:R>", end.Unix()),
			},
		})

		go func() {
			time.Sleep(end.Sub(time.Now()))
			s.GuildMemberMute(i.GuildID, userId, false)
			if err := s.GuildMemberRoleRemove(i.GuildID, userId, mute); err != nil {
				slog.Error("failed to unmute", "error", err)
			} else {
				content := "Mute is finished"
				if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: &content,
				}); err != nil {
					slog.Error("failed to edit message", "error", err)
				}
			}
		}()
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Already muted",
			},
		})
	}

	return nil
}
