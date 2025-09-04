package main

//#include <stdlib.h>
import "C"
import (
	"os"
	"unsafe"
)

//export Free
func Free(ptr *C.char) {
	C.free(unsafe.Pointer(ptr))
}

func main() {

}

func init() {
	data, err := os.ReadFile(_get_path())
	if err == nil {
		_parse(data)
	}
}
