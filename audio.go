package main

import "C"

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

type audioHandle struct {
	id       int
	streamer beep.StreamSeekCloser
}

var (
	mu            sync.Mutex
	handles       = make(map[int]audioHandle)
	nextID        = 1
	speakerInited bool
)

type FadeOut struct {
	Streamer beep.Streamer
	Volume   float64
	Step     float64
	Done     chan struct{}
}

func (f *FadeOut) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = f.Streamer.Stream(samples)
	for i := 0; i < n; i++ {
		samples[i][0] *= f.Volume
		samples[i][1] *= f.Volume
		f.Volume -= f.Step
		if f.Volume < 0 {
			f.Volume = 0
		}
	}
	if f.Volume == 0 {
		select {
		case f.Done <- struct{}{}:
		default:
		}
		return n, false
	}
	return n, ok
}
func (f *FadeOut) Err() error { return f.Streamer.Err() }

//export PlayAudio
func PlayAudio(data *C.char, length C.int, ext *C.char) C.int {
	goBytes := C.GoBytes(unsafe.Pointer(data), length)
	extension := C.GoString(ext)

	streamer, format, err := decodeAudio(goBytes, extension)
	if err != nil {
		return -1
	}

	mu.Lock()
	defer mu.Unlock()

	if !speakerInited {
		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			return -2
		}
		speakerInited = true
	}

	id := nextID
	nextID++

	handles[id] = audioHandle{id: id, streamer: streamer}

	done := make(chan bool, 1)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	go func(myID int, s beep.StreamSeekCloser, done chan bool) {
		<-done
		mu.Lock()
		defer mu.Unlock()
		s.Close()
		delete(handles, myID)
	}(id, streamer, done)

	return C.int(id)
}

//export StopAudio
func StopAudio(id C.int) {
	mu.Lock()
	handle, ok := handles[int(id)]
	if !ok {
		mu.Unlock()
		return
	}
	delete(handles, int(id))
	mu.Unlock()

	fade := &FadeOut{
		Streamer: handle.streamer,
		Volume:   1.0,
		Step:     0.001,
		Done:     make(chan struct{}, 1),
	}

	speaker.Lock()
	speaker.Play(fade)
	speaker.Unlock()

	go func(s beep.StreamSeekCloser, done chan struct{}) {
		<-fade.Done
		s.Close()
	}(handle.streamer, fade.Done)
}

//export StopAllAudio
func StopAllAudio() {
	mu.Lock()
	defer mu.Unlock()
	for id, h := range handles {
		h.streamer.Close()
		delete(handles, id)
	}
	handles = make(map[int]audioHandle)
}

func decodeAudio(data []byte, ext string) (beep.StreamSeekCloser, beep.Format, error) {
	reader := bytes.NewReader(data)
	rc := io.NopCloser(reader)
	switch ext {
	case ".mp3":
		return mp3.Decode(rc)
	case ".wav":
		return wav.Decode(rc)
	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported extension: %s", ext)
	}
}
