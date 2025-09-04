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
		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			return -1
		}
		speakerInited = true
	}
	mu.Lock()
	if len(handles) == 0 {
		nextID = 1
	}
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
		mu.Lock()
		_, ok := handles[id]
		if !ok {
			mu.Unlock()
			return
		}
		speaker.Lock()
		defer speaker.Unlock()
		streamer.Close()
		delete(handles, id)
		mu.Unlock()
	}()
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
		handle.volume.Streamer = nil
		speaker.Unlock()
		done <- true
	}()
	<-done
}

//export StopAllAudio
func StopAllAudio() {
	go func() {
		for id := 1; id <= getMaxID(handles); id++ {
			StopAudio(C.int(id))
		}
	}()
}

func getMaxID[T any](d map[int]T) int {
	res := -1
	for k := range d {
		if res < k {
			res = k
		}
	}
	return res
}

func decodeAudio(data []byte) (beep.StreamSeekCloser, beep.Format, error) {
	s, f, err := wav.Decode(bytes.NewReader(data))
	if err != nil {
		return mp3.Decode(io.NopCloser(bytes.NewReader(data)))
	}
	return s, f, nil
}
