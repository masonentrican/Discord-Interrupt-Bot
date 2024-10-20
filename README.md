# Discord Interrupt Bot

**Discord Interrupt Bot** is a bot designed to interrupt a specific user in a voice channel by playing an audio file whenever they start speaking. This bot uses the Discord API and requires a Discord bot token, target user, and an audio file to function.
## Features
- Detects when a specified user starts speaking in a voice channel.
- Interrupts the user by playing a provided `.dca` audio file.
- Works concurrently, ensuring continuous packet listening while audio plays.
- Can auto-join a specified channel or wait for the user to join.

## Installation

Clone this repository

```bash
    git clone https://github.com/your-username/discord-interrupt-bot.git
    cd discord-interrupt-bot
```

Download GO
```bash
    go mod download
```

Compile
```
    go build
```

### Prerequisites

- **Go** (made with 1.23.2)
- **Discord Bot Token** (from the [Discord Developer Portal](https://discord.com/developers/applications))
- A **target Discord user's nickname or username**.
- An **audio file in `.dca` format**.

### Convert an MP3 file to `.dca`

You can convert `.mp3` files to `.dca` using `ffmpeg` and `dca` (both tools should be installed):

```bash
ffmpeg -i yourfile.mp3 -f s16le -ar 48000 -ac 2 pipe:1 | dca > yourfile.dca
```
## Usage/Examples

```bash
./discord-interrupt-bot -t <BOT_TOKEN> -g <GUILD_ID> -n <TARGET_USERNAME> -a <AUDIO_FILE.dca> [-c <CHANNEL_ID>]
```

Required Arguments:
- t : Your Discord Bot Token.
- g : Your Guild ID (Server ID).
- n : The target user’s nickname or username in the guild.
- a : Path to the .dca audio file that will play when the user speaks.
Optional Argument:
- c : The Channel ID of the voice channel to auto-join. If not specified, the bot will join when the target user joins a voice channel. Note that the bot will not join if the user is already in the channel before running.
## How it works


1. The bot monitors the voice state of the specified user in the provided guild.
2. When the user starts speaking in any voice channel, the bot joins the channel and plays the .dca audio file.
3. If the user stops speaking for a set period (0.5 seconds), the audio stops.
## License

[GNUv3.0](https://choosealicense.com/licenses/gpl-3.0/)
Feel free to fork the repository, create a branch, and submit pull requests with improvements.


