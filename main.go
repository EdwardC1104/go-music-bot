package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/bwmarrin/discordgo"
)

var buffer = make([][]byte, 0)

func main() {
	err := godotenv.Load()

	if err != nil {
		fmt.Println("Error loading .env file", err)
		return
	}

	bot, err := discordgo.New("Bot " + os.Getenv("token"))

	if err != nil {
		panic(err)
	}

	bot.AddHandler(ready)
	bot.AddHandler(messageCreate)
	bot.AddHandler(guildCreate)

	// bot.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

	err = bot.Open()

	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	bot.Close()
	fmt.Println("Exited")
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	s.UpdateStatus(0, "music")
	fmt.Println("logged in as user " + string(s.State.User.ID))
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, "!play") {
		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			return
		}

		g, err := s.State.Guild(c.GuildID)
		if err != nil {
			return
		}

		for _, vs := range g.VoiceStates {
			if vs.UserID == m.Author.ID {
				err = playSound(s, g.ID, vs.ChannelID)
				if err != nil {
					fmt.Println("Error playing sound:", err)
				}

				return
			}
		}
	}

}

func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {

	if event.Guild.Unavailable {
		return
	}

	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			_, _ = s.ChannelMessageSend(channel.ID, "Music Bot is ready!")
			return
		}
	}
}

func playSound(s *discordgo.Session, guildID, channelID string) (err error) {
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return err
	}
	vc.Speaking(true)

	for _, buff := range buffer {
		vc.OpusSend <- buff
	}

	vc.Speaking(false)
	vc.Disconnect()
	return nil
}

func loadSound() error {

	file, err := os.Open("airhorn.dca")
	if err != nil {
		fmt.Println("Error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// Read opus frame length from dca file.
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return.
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err := file.Close()
			if err != nil {
				return err
			}
			return nil
		}

		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Read encoded pcm from dca file.
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Append encoded pcm data to the buffer.
		buffer = append(buffer, InBuf)
	}
}
