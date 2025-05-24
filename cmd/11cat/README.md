# 11cat

Command 11cat is a text-to-speech tool that converts standard input into
streaming audio using the ElevenLabs API.

The tool reads text from standard input, chunks it intelligently at natural
speech boundaries (punctuation marks and sentence breaks), and streams the
resulting audio to standard output as MP3 data.

Usage:

    11cat < input.txt > output.mp3
    echo "Hello, world!" | 11cat | mpv -
    cat book.txt | 11cat | ffplay -f mp3 -i pipe:0

The command requires the ELEVENLABS_API_KEY environment variable to be set
with a valid ElevenLabs API key.

By default, it uses the voice ID "21m00Tcm4TlvDq8ikWAM" (Rachel). You can
specify a different voice using the -voice-id flag:

    11cat -voice-id "ErXwobaYiN019PkySvjV" < input.txt > output.mp3

The tool uses WebSocket streaming for real-time text-to-speech conversion,
which allows it to start producing audio output before all input text has
been read. This makes it suitable for piping large amounts of text or
integrating with other command-line tools.

Example pipeline for reading a markdown file aloud:

    pandoc -t plain document.md | 11cat | mpv -

Command 11cat turns stdin into a stream of audio
