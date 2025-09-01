package main

import (
	"C"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

var (
	d      *zstd.Decoder
	_table map[string][]int64
)

//export GetResource
func GetResource(a string) []byte {
	return _GetResource(a)
}

func _GetResource(a string) []byte {
	i := _table[a]
	l := i[0]
	res := make([]byte, l)
	f, err := os.Open(fmt.Sprintf("datas/data%d.dat", i[2]))
	if err != nil {
		return nil
	}
	defer f.Close()
	_, err = f.Seek(i[1], io.SeekStart)
	if err != nil {
		return nil
	}
	_, err = f.Read(res)
	if err != nil {
		return nil
	}
	return res
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
	raw := make([]byte, len(b))
	for _, v := range b {
		raw = append(raw, ^v)
	}
	json.Unmarshal(raw, &_table)
}

func init() {
	data, err := os.ReadFile(_get_path())
	if err != nil {
		_parse(data)
	}
}

func main() {

}
