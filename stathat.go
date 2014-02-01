package main

import (
	"github.com/stathat/go"
)

var enableStatHat = false
var stathatUserKey string

// StatCount adds count to a specific metric.
func StatCount(statKey string, count int) {
	if enableStatHat {
		stathat.PostEZCount(statKey, stathatUserKey, count)
	}
}

// StatValue records value for a specific metric.
func StatValue(statKey string, value float64) {
	if enableStatHat {
		stathat.PostEZValue(statKey, stathatUserKey, value)
	}
}
