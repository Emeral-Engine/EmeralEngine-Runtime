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
	volume   *effects.Volume
}

var (
	mu            sync.RWMutex
	handles       = make(map[int]*audioHandle)
	nextID        = 1
	speakerInited bool
)

const (
	SR beep.SampleRate = 48000
)

//export PlayAudio
func PlayAudio(h *C.char) C.int {
	return _PlayAudio(h, false)
}

//export PlayLoopAudio
func PlayLoopAudio(h *C.char) C.int {
	return _PlayAudio(h, true)
}

func _PlayAudio(h *C.char, loop bool) C.int {
	length := C.int(0)
	b := GetResourceData(h, &length)
	buf := C.GoBytes(unsafe.Pointer(b), length)
	return _PlayAudioWithBytes(buf, loop)
}

//export PlayAudioWithBytes
func PlayAudioWithBytes(b *C.char, l C.int) C.int {
	return _PlayAudioWithBytes(C.GoBytes(unsafe.Pointer(b), l), false)
}

//export PlayLoopAudioWithBytes
func PlayLoopAudioWithBytes(b *C.char, l C.int) C.int {
	return _PlayAudioWithBytes(C.GoBytes(unsafe.Pointer(b), l), true)
}

func _PlayAudioWithBytes(buf []byte, loop bool) C.int {
	streamer, format, err := decodeAudio(buf)
	if err != nil {
		return -1
	}
	if !speakerInited {
		if err := speaker.Init(SR, SR.N(time.Second/10)); err != nil {
			return -1
		}
		speakerInited = true
	}
	mu.Lock()
	id := nextID
	nextID++
	mu.Unlock()
	var (
		vol *effects.Volume
	)
	if loop {
		vol = &effects.Volume{
			Streamer: beep.Loop(-1, streamer),
			Base:     2,
			Volume:   0,
			Silent:   false,
		}
	} else {
		vol = &effects.Volume{
			Streamer: streamer,
			Base:     2,
			Volume:   0,
			Silent:   false,
		}
	}
	mu.Lock()
	handles[id] = &audioHandle{id: id, streamer: streamer, sr: format.SampleRate, volume: vol}
	mu.Unlock()
	done := make(chan bool, 1)
	speaker.Play(beep.Seq(vol, beep.Callback(func() {
		done <- true
	})))
	go func() {
		<-done
		_delStream(id)
	}()
	return C.int(id)
}

func _delStream(id int) {
	mu.Lock()
	speaker.Lock()
	h, ok := handles[id]
	if ok {
		h.streamer.Close()
		h.volume.Streamer = nil
		delete(handles, id)
	}
	speaker.Unlock()
	mu.Unlock()
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
	done := make(chan bool)
	steps := 60
	stepDelay := 1 * time.Second / time.Duration(steps)
	stepSize := -6.0 / float64(steps)
	go func() {
		for i := 0; i < steps; i++ {
			speaker.Lock()
			handle.volume.Volume += stepSize
			speaker.Unlock()
			time.Sleep(stepDelay)
		}
		_delStream(int(id))
		done <- true
	}()
	<-done
}

//export StopAllAudio
func StopAllAudio() {
	mu.RLock()
	h := handles
	mu.RUnlock()
	go func() {
		for i := range h {
			StopAudio(C.int(i))
		}
	}()
}

func decodeAudio(data []byte) (beep.StreamSeekCloser, beep.Format, error) {
	s, f, err := wav.Decode(bytes.NewReader(data))
	if err != nil {
		return mp3.Decode(io.NopCloser(bytes.NewReader(data)))
	}
	return s, f, nil
}
