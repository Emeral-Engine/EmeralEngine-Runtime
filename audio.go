package main

import "C"

import (
	"bytes"
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
)

type audioHandle struct {
	id       int
	sr       beep.SampleRate
	streamer beep.StreamSeekCloser
	ctrl     *beep.Ctrl
	volume   *effects.Volume
}

var (
	mu            sync.Mutex
	handles       = make(map[int]audioHandle)
	nextID        = 1
	speakerInited bool
)

//export PlayAudio
func PlayAudio(h *C.char) C.int {
	length := C.int(0)
	b := GetResourceData(h, &length)
	buf := C.GoBytes(unsafe.Pointer(b), length)
	return _PlayAudioWithBytes(buf)
}

//export PlayAudioWithBytes
func PlayAudioWithBytes(b *C.char, l C.int) C.int {
	return _PlayAudioWithBytes(C.GoBytes(unsafe.Pointer(b), l))
}

func _PlayAudioWithBytes(buf []byte) C.int {
	streamer, format, err := decodeAudio(buf)
	if err != nil {
		panic(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !speakerInited {
		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			panic(err)
		}
		speakerInited = true
	}

	id := nextID
	nextID++
	vol := &effects.Volume{
		Streamer: streamer,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}
	ctrl := &beep.Ctrl{
		Streamer: vol,
		Paused:   false,
	}
	done := make(chan bool, 1)
	handles[id] = audioHandle{id: id, streamer: streamer, sr: format.SampleRate, volume: vol, ctrl: ctrl}
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		done <- true
	})))
	go func(myID int, s beep.StreamSeekCloser, done chan bool) {
		<-done
		speaker.Lock()
		defer speaker.Unlock()
		s.Close()
		delete(handles, myID)
	}(id, streamer, done)
	return C.int(id)
}

//export StopAudio
func StopAudio(id C.int) {
	handle, ok := handles[int(id)]
	if !ok {
		mu.Unlock()
		return
	}
	delete(handles, int(id))
	done := make(chan bool)
	steps := 30
	stepDelay := 3 * time.Second / time.Duration(steps)
	stepSize := -3.0 / float64(steps) // -3dB ずつ下げる

	go func() {
		for i := 0; i < steps; i++ {
			speaker.Lock()
			handle.volume.Volume += stepSize
			speaker.Unlock()
			time.Sleep(stepDelay)
		}
		speaker.Lock()
		handle.streamer.Close()
		handle.ctrl.Streamer = nil
		speaker.Unlock()
		done <- true
	}()
	<-done
}

//export StopAllAudio
func StopAllAudio() {
	mu.Lock()
	defer mu.Unlock()
	for id := range handles {
		StopAudio(C.int(id))
	}
}

func decodeAudio(data []byte) (beep.StreamSeekCloser, beep.Format, error) {
	reader := bytes.NewReader(data)
	rc := io.NopCloser(reader)
	s, f, err := wav.Decode(rc)
	if err != nil {
		return mp3.Decode(rc)
	}
	return s, f, nil
}
