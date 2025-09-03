package main

//#include <string.h>
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
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

func decodeAudio(data []byte) (beep.StreamSeekCloser, beep.Format, error) {
	reader := bytes.NewReader(data)
	rc := io.NopCloser(reader)
	s, f, err := wav.Decode(rc)
	if err != nil {
		return mp3.Decode(rc)
	}
	return s, f, nil
}

func main() {
	fmt.Println(PlayAudio(C.CString("ac120b70cf3e1f930a658f61aa728636a8767abab1752ad1f8f64cf3ade909b7")))
}
