/*
Interrupts a targeted discord user with audio everytime they talk

Inspired by a short video by @aaronr5 on TikTok


===============================
Console parameters
===============================

REQUIRED

Bot Token:      -t    string   (eg MGdg93jf9.gSDJ9fg39jgg3ghh.gGGGDJHEA0_KCVKVgDzk165)
	Your discord bot token from the developer portal

Guild ID:       -g    string   (eg 246120986422723874624306)
	Your Guild ID (Server ID, found by right clicking the server with developer options on)

Display Name:   -n    string   (eg TheVictim)
	Target victim display name (Nickname or defaults to global display name)

Audio File:     -a    string   (eg annoying.dca)
	File path to .dca audio relative to project root. Reccomended to keep it in this folder
	Mp3 can be converted to .dca with ffmpeg command:    ffmpeg -i test.mp3 -f s16le -ar 48000 -ac 2 pipe:1 | dca > test.dca


OPTIONAL

Channel ID:     -c    string   (eg 35098234609283304298235)
	Provide a channel ID if you want to have the bot immidiately join a channel rather than wait for victim to join




Author: Mason Entrican (mason@entrican.com)

Changelog
	Version 1.0 (2024-10-24)

*/

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	Token         string
	GuildID       string
	ChannelID     string
	Nickname      string
	UserID        string
	AudioFilePath string

	AudioTimer     *time.Timer
	AudioBuffer    = make([][]byte, 0)
	isAudioPlaying bool

	UserSSRCMap = make(map[uint32]string) // SSRC to UserID mapping
)

// Flags for command line params
func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.StringVar(&GuildID, "g", "", "Target Guild ID")
	flag.StringVar(&Nickname, "n", "", "Guild nickname of user (falls back to disc username)")
	flag.StringVar(&ChannelID, "c", "", "Target Channel if member is already in channel (req if user is already in voice)")
	flag.StringVar(&AudioFilePath, "a", "blah.dca", "Audio file path (must be dca)")
	flag.Parse()
}

func main() {
	// Load the dca sound into audio buffer
	err := loadSound()
	if err != nil {
		log.Fatal("Failed to load sound into audio buffer")
		return
	}
	log.Println("Loaded audio into buffer")

	// Create session
	s, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Println("error creating Discord session:", err)
		return
	}
	defer s.Close()
	log.Println("Created Discord Session")

	// Handle voice state updates so we have state cache
	s.AddHandler(voiceStateUpdate)

	// Open session
	s.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMembers | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildPresences)

	err = s.Open()
	if err != nil {
		log.Println("error opening connection:", err)
		return
	}
	log.Println("Opened discord session")

	// Fetch guild members
	members, err := s.GuildMembers(GuildID, "", 1000)
	if err != nil {
		log.Fatalf("Error retrieving guild members: %v", err)
	}

	// Find the memberID for target user by nickname
	for _, member := range members {
		if member.DisplayName() == Nickname || strings.EqualFold(member.User.Username, Nickname) {
			UserID = member.User.ID
			log.Printf("Found user %s with ID %s", Nickname, UserID)
			break
		}
	}

	// Quit out if member not found
	if UserID == "" {
		log.Printf("Could not locate user by nickname %s", Nickname)
		return
	}

	if ChannelID != "" {
		targetVoiceChannel, err := s.ChannelVoiceJoin(GuildID, ChannelID, false, false)
		if err != nil || targetVoiceChannel == nil {
			log.Fatalf("Failed to join voice channel id %s - %v", ChannelID, err)
			return
		}

		handleVoice(targetVoiceChannel)
	}

	// Wait for a termination signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

}

func voiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	// Fetch the member's info
	member, err := s.GuildMember(vs.GuildID, vs.UserID)
	if err != nil {
		log.Printf("Error fetching member: %v", err)
		return
	}

	// Check if the user's nickname or username matches the target nickname
	if member.DisplayName() == Nickname || strings.EqualFold(member.User.Username, Nickname) {
		if vs.ChannelID != "" {
			log.Printf("User %s is in voice channel %s - Attempting to Join", Nickname, vs.ChannelID)

			// Join the voice channel
			targetVoiceChannel, err := s.ChannelVoiceJoin(GuildID, vs.ChannelID, false, false)
			if err != nil {
				log.Fatalf("Failed to join voice channelid %s - %v", vs.ChannelID, err)
			}

			// Bind speaking handler to voice channel and use to map packet SSRC to userID
			targetVoiceChannel.AddHandler(func(vc *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
				log.Printf("Updating SSRC mapping for UserID %s | SSRC %v", vs.UserID, vs.SSRC)
				UserSSRCMap[uint32(vs.SSRC)] = vs.UserID
			})

			handleVoice(targetVoiceChannel)
		}
	}
}

func handleVoice(vc *discordgo.VoiceConnection) {
	// Run packet receiving loop in the main routine
	go func() {
		for p := range vc.OpusRecv {
			log.Println("Packet recieved: ", p.SSRC, " | ", p.Type, " | ", p.Sequence)

			// Check if this SSRC is new, if so, map it
			if _, err := UserSSRCMap[p.SSRC]; !err {
				UserSSRCMap[p.SSRC] = "UKNOWN"
				log.Printf("New SSRC %d. Mapped to unknown", p.SSRC)
			}

			// Check if this packet came from target user
			if UserSSRCMap[p.SSRC] == UserID {
				log.Println("Target User is speaking")

				if !isAudioPlaying {
					go startAudio(vc) // Play audio in a seperate goroutine
				}

				// Reset the audio timer
				if AudioTimer != nil {
					AudioTimer.Stop()
				}

				AudioTimer = time.AfterFunc(time.Second/2, func() {
					log.Println("Packets stopped for 1 second. Stopping audio")
					stopAudio(vc)
				})
			}
		}
	}()
}

func startAudio(vc *discordgo.VoiceConnection) {
	log.Println("Audio started")
	isAudioPlaying = true

	vc.Speaking(true)

	for _, buff := range AudioBuffer {
		if !isAudioPlaying {
			break // Stop playback of buffer if audio is marked no play
		}

		vc.OpusSend <- buff

	}
}

func stopAudio(vc *discordgo.VoiceConnection) {
	log.Println("Audio STOPPED")
	isAudioPlaying = false

	vc.Speaking(false)
}

// Load dca audio file into buffer
func loadSound() error {
	file, err := os.Open(AudioFilePath)
	if err != nil {
		fmt.Println("Error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// Read opus frame length from dca file
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return
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

		// Read encoded pcm from dca file
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Append encoded pcm data to the buffer
		AudioBuffer = append(AudioBuffer, InBuf)
	}

}
