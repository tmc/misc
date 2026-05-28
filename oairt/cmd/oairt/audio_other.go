//go:build !darwin

package main

func NewAudioPlayer(state *AppState) AudioEngine {
	return NewFFplayPlayer(state)
}

func checkAudioDependencies() error { return checkFFplay() }
