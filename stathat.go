package main

import (
	"github.com/stathat/go"
)

var enableStatHat = false
var stathatUserKey string

func StatCount(statKey string, count int) {
	if enableStatHat {
		stathat.PostEZCount(statKey, stathatUserKey, count)
	}
}

func StatValue(statKey string, value float64) {
	if enableStatHat {
		stathat.PostEZValue(statKey, stathatUserKey, value)
	}
}
