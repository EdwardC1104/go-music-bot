package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/jonas747/dca"
	"github.com/kkdai/youtube/v2"
)

var voiceConnection *discordgo.VoiceConnection
var songQueue = make(chan string)
var streamer *dca.StreamingSession

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

	// bot.AddHandler(ready)
	bot.AddHandler(messageCreate)

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

// func ready(s *discordgo.Session, event *discordgo.Ready) {
// 	// s.UpdateStatus(0, "for Messages")
// 	fmt.Println("logged in as user " + string(s.State.User.ID))
// }

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, "!join") {
		go join(s, m)
	} else if strings.HasPrefix(m.Content, "!leave") {
		go leave()
	} else if strings.HasPrefix(m.Content, "!play ") {
		go add(m.Content)
	} else if strings.HasPrefix(m.Content, "!skip ") {
		go skip()
	}

}

func join(s *discordgo.Session, m *discordgo.MessageCreate) {
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
			vc, err := s.ChannelVoiceJoin(g.ID, vs.ChannelID, false, true)
			if err != nil {
				fmt.Println("Error playing sound:", err)
				return
			}

			go playback(vc)
			voiceConnection = vc
		}
	}
}

func leave() {
	voiceConnection.Disconnect()
}

func add(messageContent string) {
	queryText := strings.ReplaceAll(messageContent, "!play ", "")

	cmd := exec.Command("youtube-dl", "--get-id", "ytsearch1:"+queryText+"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + string(output))
		return
	}

	songQueue <- strings.TrimSpace(string(output))
}

func playback(vc *discordgo.VoiceConnection) {
	for {
		songUrl := <-songQueue
		fmt.Println(songUrl)
		vc.Speaking(true)

		options := dca.StdEncodeOptions
		options.RawOutput = true
		options.Bitrate = 96
		options.Application = "audio"
		options.FrameDuration = 20
		options.BufferedFrames = 2048
		options.Threads = 8

		videoID := songUrl
		client := youtube.Client{}

		video, err := client.GetVideo(videoID)
		if err != nil {
			fmt.Println("Error getting video infomation", err)
			return
		}

		format := video.Formats.WithAudioChannels()

		downloadURL, err := client.GetStreamURL(video, &format[0])
		if err != nil {
			fmt.Println("Error getting video stream url", err)
			return
		}

		encodingSession, err := dca.EncodeFile(downloadURL, options)
		if err != nil {
			fmt.Println("Error encoding the audio", err)
			return
		}
		defer encodingSession.Cleanup()

		done := make(chan error)
		dca.NewStream(encodingSession, vc, done)
		err = <-done
		if err != nil && err != io.EOF {
			fmt.Println("Error streaming to Discord", err)
			return
		}

		vc.Speaking(false)
	}
}

func skip() {
	streamer.SetPaused(true)
}
