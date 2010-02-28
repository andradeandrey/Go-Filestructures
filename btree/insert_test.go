package btree

import "testing"
import "fmt"
import . "block/keyblock"
import . "block/byteslice"

const ORDER_2 = 45
const ORDER_3 = 65
const ORDER_4 = 85
const ORDER_5 = 105

func TestOrder(t *testing.T) {
    fmt.Println("\n\n\n------  TestOrder  ------")
    order2 := makebtree(ORDER_2)
    if order2.node.KeysPerBlock() != 2 {
        t.Error("order 2 btree not order 2 it is order", order2.node.KeysPerBlock())
    }
    cleanbtree(order2)
    
    order3 := makebtree(ORDER_3)
    if order3.node.KeysPerBlock() != 3 {
        t.Error("order 2 btree not order 2 it is order", order3.node.KeysPerBlock())
    }
    cleanbtree(order3)
    
    order4 := makebtree(ORDER_4)
    if order4.node.KeysPerBlock() != 4 {
        t.Error("order 2 btree not order 2 it is order", order4.node.KeysPerBlock())
    }
    cleanbtree(order4)
    
    order5 := makebtree(ORDER_5)
    if order5.node.KeysPerBlock() != 5 {
        t.Error("order 2 btree not order 2 it is order", order5.node.KeysPerBlock())
    }
    cleanbtree(order5)
}

func makerec(self *BTree, key ByteSlice) *Record {
    r := self.node.NewRecord(key)
    for i, f := range rec {
        r.Set(uint32(i), f)
    }
    return r
}

func insert(self *BTree, a *KeyBlock, key ByteSlice) bool {
    r := makerec(self, key)
    _, b := a.Add(r)
    return b
}

func fill_block(self *BTree, a *KeyBlock, t *testing.T, skip int) {
    n := self.node.KeysPerBlock()
    if skip < n { n++ }
    p_ := uint32(0)
    for i := uint32(0); int(i) < n; i++ {
        if int(i) == skip { p_ = 1; continue }
        if !insert(self, a, ByteSlice32(i)) {
            t.Errorf("failed inserting ith, %v, value in block of order %v", i+1, self.node.KeysPerBlock())
        }
        if i-p_ == 0 {
            a.InsertPointer(int(i-p_), ByteSlice64(uint64(i-p_+1)))
        }
        a.InsertPointer(int(i-p_+1), ByteSlice64(uint64(i-p_+2)))
    }
}

func testBalanceBlocks(self *BTree, t *testing.T) {
    a := self.getblock(self.root)
    b := self.allocate()
    
    fill_block(self, a, t, self.node.KeysPerBlock())
    
    self.balance_blocks(a, b)
    if a.RecordCount() > b.RecordCount() {
        t.Errorf("a.RecordCount() > b.RecordCount()")
    }
    if a.PointerCount() < b.PointerCount() {
        t.Errorf("a.PointerCount() < b.PointerCount()")
    }
    if a.RecordCount() != b.RecordCount() && a.PointerCount()+1 != b.RecordCount()+1 {
        t.Errorf("record balance off")
    }
    if a.PointerCount() != b.PointerCount() && a.PointerCount() != b.PointerCount()+1 {
        t.Errorf("pointer balance off")
    }
}

func TestBalanceBlocksO2(t *testing.T) {
    fmt.Println("\n\n\n------  TestBalanceBlocksO2  ------")
    self := makebtree(ORDER_2)
    defer cleanbtree(self)
    testBalanceBlocks(self, t)
}

func TestBalanceBlocksO3(t *testing.T) {
    fmt.Println("\n\n\n------  TestBalanceBlocksO3  ------")
    self := makebtree(ORDER_3)
    defer cleanbtree(self)
    testBalanceBlocks(self, t)
}

func TestBalanceBlocksO4(t *testing.T) {
    fmt.Println("\n\n\n------  TestBalanceBlocksO4  ------")
    self := makebtree(ORDER_4)
    defer cleanbtree(self)
    testBalanceBlocks(self, t)
}

func TestBalanceBlocksO5(t *testing.T) {
    fmt.Println("\n\n\n------  TestBalanceBlocksO5  ------")
    self := makebtree(ORDER_5)
    defer cleanbtree(self)
    testBalanceBlocks(self, t)
}




func validateSimpleSplit(self *BTree, a *KeyBlock, c *Record, dirty *dirty_blocks, t *testing.T) {
    
    b, rec, ok := self.split(a, c, nil, dirty)
    
    if !ok {
        t.Error("Could not split a on c")
    }
    
    i := 0
    for ; i < int(a.RecordCount()); i++ {
        r, left, right, ok := a.Get(i)
        if !ok {
            t.Error("Error getting rec at index ", i)
        }
        if int(r.GetKey().Int32()) != i {
            t.Errorf("138 Error key, %v, does not equal %v", r.GetKey().Int32(), i)
        }
        if left.Int64() != uint64(i+1) {
            t.Errorf("141 Error left, %v, does not equal 0x%x", right, i+1)
        }
        if right.Int64() != uint64(i+2) {
            t.Errorf("144 Error right, %v, does not equal 0x%x", right, i+2)
        }
    }
    
    if int(rec.GetKey().Int32()) != i {
        t.Errorf("149 Error key, %v, does not equal %v", rec.GetKey().Int32(), i)
    }
    i++
    
    
    for j := 0; j < int(b.RecordCount()); j++ {
        r, left, right, ok := b.Get(j)
        if !ok {
            t.Error("Error getting rec at index ", i)
        }
        if int(r.GetKey().Int32()) != i {
            t.Errorf("160 Error key, %v, does not equal %v", r.GetKey().Int32(), i)
        }
        if left.Int64() != uint64(i+1) {
            t.Errorf("163 Error left, %v, does not equal 0x%x", right, i+1)
        }
        if j+1 != int(b.RecordCount()) && right.Int64() != uint64(i+2) {
            t.Errorf("166 Error right, %v, does not equal 0x%x", right, i+2)
        }
        i++
    }
}

func testSimpleSplit(self *BTree, t *testing.T) {
    fmt.Println("case 1")
    dirty := new_dirty_blocks(100)
    a := self.allocate()
    dirty.insert(a)
    fill_block(self, a, t, self.node.KeysPerBlock())
    validateSimpleSplit(self, a, makerec(self, ByteSlice32(uint32(self.node.KeysPerBlock()))), dirty, t)
    
    
    fmt.Println("case 2")
    a = self.allocate()
    dirty.insert(a)
    fill_block(self, a, t, self.node.KeysPerBlock()>>1)
    validateSimpleSplit(self, a, makerec(self, ByteSlice32(uint32(self.node.KeysPerBlock())>>1)), dirty, t)
    
    
    fmt.Println("case 3")
    a = self.allocate()
    dirty.insert(a)
    fill_block(self, a, t, 0)
    validateSimpleSplit(self, a, makerec(self, ByteSlice32(0)), dirty, t)
}

func TestSimpleSplitO2(t *testing.T) {
    fmt.Println("\n\n\n------  TestSimpleSplitO2  ------")
// func (self *BTree) split(block *KeyBlock, rec *Record, nextb *KeyBlock, dirty *dirty_blocks) (*KeyBlock, *Record, bool) {
    self := makebtree(ORDER_2)
    defer cleanbtree(self)
    testSimpleSplit(self, t)
}


func TestSimpleSplitO3(t *testing.T) {
    fmt.Println("\n\n\n------  TestSimpleSplitO3  ------")
// func (self *BTree) split(block *KeyBlock, rec *Record, nextb *KeyBlock, dirty *dirty_blocks) (*KeyBlock, *Record, bool) {
    self := makebtree(ORDER_3)
    defer cleanbtree(self)
    testSimpleSplit(self, t)
}

func TestSimpleSplitO4(t *testing.T) {
    fmt.Println("\n\n\n------  TestSimpleSplitO4  ------")
// func (self *BTree) split(block *KeyBlock, rec *Record, nextb *KeyBlock, dirty *dirty_blocks) (*KeyBlock, *Record, bool) {
    self := makebtree(ORDER_4)
    defer cleanbtree(self)
    testSimpleSplit(self, t)
}

func TestSimpleSplitO5(t *testing.T) {
    fmt.Println("\n\n\n------  TestSimpleSplitO5  ------")
// func (self *BTree) split(block *KeyBlock, rec *Record, nextb *KeyBlock, dirty *dirty_blocks) (*KeyBlock, *Record, bool) {
    self := makebtree(ORDER_5)
    defer cleanbtree(self)
    testSimpleSplit(self, t)
}



func constructCompleteLevel2(self *BTree, order, skip int) {
    dirty := new_dirty_blocks(100)
    n := order*(order+2)
    if (skip <= n) { n++ }
    root := self.getblock(self.root)
    dirty.insert(root)
    cur := self.allocate()
    dirty.insert(cur)
    for i := 0; i < n; i++ {
        if (i+1 == skip) { continue }
        rec :=  makerec(self, ByteSlice32(uint32(i+1)))
        if cur.Full() {
            root.InsertPointer(int(root.PointerCount()), cur.Position())
            cur = self.allocate() 
            dirty.insert(cur)
            root.Add(rec)
        } else {
            cur.Add(rec)
        }
    }
    root.InsertPointer(int(root.PointerCount()), cur.Position())
    dirty.sync()
    self.height = 2
}

func verify_tree(self *BTree, t *testing.T) {
    var traverse func(*KeyBlock) int
    j := 1
    traverse = func(block *KeyBlock) int {
        i := 0
        for ; i < int(block.RecordCount()); i++ {
            rec, _, _, ok := block.Get(i)
            if !ok {
                t.Errorf("could not get rec, %v, from block with %v records", i, block.RecordCount())
            }
            if p, ok := block.GetPointer(i); ok {
                nblock := self.getblock(p)
                if nblock == nil {
                    t.Errorf("nil block returned by self.getblock(p)", i, block.RecordCount())
                }
                traverse(nblock)
            }
            if !rec.GetKey().Eq(ByteSlice32(uint32(j))) {
                t.Errorf("block invalid expecting key %v got %v", j, rec.GetKey().Int32())
            }
            j++
        }
        if p, ok := block.GetPointer(i); ok {
            nblock := self.getblock(p)
            if nblock == nil {
                t.Errorf("nil block returned by self.getblock(p)", i, block.RecordCount())
            }
            traverse(nblock)
        }
        return j
    }
    order := self.node.KeysPerBlock()
    j = traverse(self.getblock(self.root))
    if j-1 != order*(order+2) + 1 {
        t.Errorf("tree missing a key", j-1,order*(order+2) + 1 )
    }
}

// the more general split is easiest to test by running an insert into the tree an verifying
// it is the correct tree.

func TestSplitO2(t *testing.T) {
    fmt.Println("\n\n\n------  TestSplitO2  ------")
    self := makebtree(ORDER_2)
    defer cleanbtree(self)
    skip := 4
    constructCompleteLevel2(self, 2, skip)
    fmt.Println(self)
    self.Insert(ByteSlice32(uint32(skip)), rec)
    verify_tree(self, t)
}

func TestSplitO3(t *testing.T) {
    fmt.Println("\n\n\n------  TestSplitO3  ------")
    self := makebtree(ORDER_3)
    defer cleanbtree(self)
    skip := 4
    constructCompleteLevel2(self, 3, skip)
    fmt.Println(self)
    self.Insert(ByteSlice32(uint32(skip)), rec)
    verify_tree(self, t)
}

func TestSplitO4(t *testing.T) {
    fmt.Println("\n\n\n------  TestSplitO4  ------")
    self := makebtree(ORDER_4)
    defer cleanbtree(self)
    skip := 4
    constructCompleteLevel2(self, 4, skip)
    fmt.Println(self)
    self.Insert(ByteSlice32(uint32(skip)), rec)
    verify_tree(self, t)
}

func TestSplitO5(t *testing.T) {
    fmt.Println("\n\n\n------  TestSplitO5  ------")
    self := makebtree(ORDER_5)
    defer cleanbtree(self)
    skip := 4
    constructCompleteLevel2(self, 5, skip)
    fmt.Println(self)
    self.Insert(ByteSlice32(uint32(skip)), rec)
    verify_tree(self, t)
}
