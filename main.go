package main

//#include <stdlib.h>
import "C"
import (
	"unsafe"
)

//export Free
func Free(ptr *C.char) {
	C.free(unsafe.Pointer(ptr))
}

/*
func main() {

}
*/
