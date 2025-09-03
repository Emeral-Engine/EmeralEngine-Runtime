package main

import "C"
import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

var (
	_table map[string][]int64
)

//export GetResourceData
func GetResourceData(h *C.char, length *C.int) *C.char {
	return _GetResource(h, length)
}

func _GetResource(_h *C.char, length *C.int) *C.char {
	h := C.GoString(_h)
	i := _table[h]
	b := make([]byte, i[0])
	f, err := os.Open(fmt.Sprintf(_get_path2(), i[2]))
	if err != nil {
		return nil
	}
	defer f.Close()
	_, err = f.Seek(i[1], io.SeekStart)
	if err != nil {
		return nil
	}
	_, err = f.Read(b)
	if err != nil {
		return nil
	}
	b[0] = 40
	b[1] = 181
	d, err := zstd.NewReader(nil)
	if err != nil {
		return nil
	}
	defer d.Close()
	res, err := d.DecodeAll(b, nil)
	if err != nil {
		return nil
	}
	*length = C.int(len(res))
	ptr := C.malloc(C.size_t(len(res)))
	if ptr == nil {
		*length = 0
		return nil
	}
	copy((*[1 << 30]byte)(ptr)[:len(res):len(res)], res)
	return (*C.char)(ptr)
}

func _get_path2() string {
	a := []int{
		100, 97, 116, 97, 115, 47, 100, 97, 116, 97, 37, 100, 46, 100, 97, 116,
	}
	r := make([]rune, 16)
	for i, b := range a {
		r[i] = rune(b)
	}
	return string(r)
}

func _get_path() string {
	a := []int{
		100, 97, 116, 97, 115, 47, 100, 97, 116, 97, 46, 100, 97, 116,
	}
	r := make([]rune, 14)
	for i, b := range a {
		r[i] = rune(b)
	}
	return string(r)
}

func _parse(b []byte) {
	var raw []byte
	for _, v := range b {
		raw = append(raw, ^v)
	}
	json.Unmarshal(raw, &_table)
}

func init() {
	data, err := os.ReadFile(_get_path())
	if err == nil {
		_parse(data)
	}
}
