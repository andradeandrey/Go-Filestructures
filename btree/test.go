package main

import "fmt"
import . "btree"
// import . "block/file"
// import . "block/keyblock"
// import . "block/buffers"
import . "block/byteslice"

func main() {
    fmt.Println("test2 yoyo")
    btree, _ := NewBTree("hello.btree", 4, &([3]uint32{1,1,2}))
    rec := &([3][]byte{&[1]byte{1},&[1]byte{1},&[2]byte{1,2}})
//     fmt.Println(btree)
    fmt.Println(btree.Insert(ByteSlice32(1), rec))
    fmt.Println(btree.Insert(ByteSlice32(2), rec))
    fmt.Println(btree.Insert(ByteSlice32(3), rec))
}
