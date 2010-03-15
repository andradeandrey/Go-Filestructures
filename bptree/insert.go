package bptree

import "fmt"
import "os"
import "rand"
import "log"
import . "block/byteslice"
import . "block/keyblock"
import "block/dirty"

type tmprec struct {
    exdim  *BlockDimensions
    indim  *BlockDimensions
    key    ByteSlice
    record []ByteSlice
}

func pkg_rec(bptree *BpTree, key ByteSlice, rec []ByteSlice) (*tmprec, bool) {
    if !bptree.ValidateKey(key) || !bptree.ValidateRecord(rec) {
        return nil, false
    }
    self := new(tmprec)
    self.exdim = bptree.external
    self.indim = bptree.internal
    self.key = key
    self.record = rec
    return self, true
}

func rec_to_tmp(bptree *BpTree, rec *Record) *tmprec {
    self := new(tmprec)
    self.exdim = bptree.external
    self.indim = bptree.internal
    self.key = rec.GetKey()
    self.record = make([][]byte, rec.Fields())
    for i := 0; i < int(rec.Fields()); i++ {
        self.record[i] = rec.Get(uint32(i))
    }
    return self
}

func (self *tmprec) makerec(rec *Record) *Record {
    for i, f := range self.record {
        rec.Set(uint32(i), f)
    }
    return rec
}

func (self *tmprec) external() *Record { return self.makerec(self.exdim.NewRecord(self.key)) }

func (self *tmprec) internal() *Record { return self.indim.NewRecord(self.key) }

func (self *tmprec) String() string {
    if self == nil {
        return "<nil tmprec>"
    }
    s := "tmprec:\n{\n"
    s += fmt.Sprintln("  exdim:", self.exdim)
    s += fmt.Sprintln("  indim:", self.indim)
    s += fmt.Sprintln("  key:", self.key)
    s += fmt.Sprintln("  record:", self.record)
    s += "}\n"
    return s
}

/*
   balance blocks takes two keyblocks full, and empty and balances the records between them. full must be full
   empty must be empty
*/
func (self BpTree) balance_blocks(full *KeyBlock, empty *KeyBlock) {
    n := int(full.MaxRecordCount())
    m := n >> 1
    if n%2 == 0 { m -= 1}
    for j := n - 1; j > m; j-- {
        if r, _, _, ok := full.Get(j); !ok {
            fmt.Printf("could not get index j<%v> from block: %v", j, full)
            os.Exit(5)
            return
        } else {
            if !full.RemoveAtIndex(j) {
                fmt.Printf("could not remove index j<%v> from block: %v", j, full)
                os.Exit(5)
                return
            }
            empty.Add(r)
        }
        if p, ok := full.GetPointer(j + 1); ok {
            empty.InsertPointer(0, p)
        }
        full.RemovePointer(j + 1)
    }
}

func (self *BpTree) split(a *KeyBlock, rec *tmprec, nextb *KeyBlock, dirty *dirty.DirtyBlocks) (*KeyBlock, *tmprec, bool) {
    b, r := func() (*KeyBlock, *Record) {
        if a.Mode() == self.external.Mode {
            return self.allocate(self.external), rec.external()
        }
        return self.allocate(self.internal), rec.internal()
    }()
    dirty.Insert(b)
    var split_rec *Record
    var return_rec *Record
    success := true
    {
        i, _, _, _, _ := a.Find(r.GetKey())
        n := int(a.MaxRecordCount()) + 1
        m := n >> 1
        //     fmt.Println("m=", m)
        if m > i {
            split_rec, _, _, _ = a.Get(m - 1)
            a.RemoveAtIndex(m - 1)
            if _, ok := a.Add(r); !ok {
                fmt.Println("Inserting record into block failed PANIC")
                os.Exit(3)
            }
        } else if m < i {
            split_rec, _, _, _ = a.Get(m)
            a.RemoveAtIndex(m)
            if _, ok := a.Add(r); !ok {
                fmt.Println("Inserting record into block failed PANIC")
                os.Exit(3)
            }
        } else {
            split_rec = r
        }
    }
    self.balance_blocks(a, b)
    var block *KeyBlock
    return_rec = split_rec
    if a.MaxRecordCount()%2 == 0 {
        if rand.Float() > 0.5 {
            block = a
            if rec, _, _, ok := b.Get(0); !ok {
                fmt.Println("Could not get the first record from block b PANIC")
                os.Exit(3)
            } else {
                return_rec = rec
            }
        } else {
            block = b
        }
    } else {
        block = b
    }
    if i, ok := block.Add(split_rec); !ok {
        success = false
    } else {
        if block.Mode()&POINTERS == POINTERS && nextb != nil {
            success = block.InsertPointer(i, nextb.Position())
        } else if block.Mode()&POINTERS == 0 && nextb != nil {
            log.Exit("tried to set a pointer on a block with no pointers")
        }
    }
    return b, rec_to_tmp(self, return_rec), success
}

// notes:
//     for allocation in case of split we may always be able to allocate the type of block being split
//     except in the case of a root split in which case the new root is always a internal node
func (self *BpTree) insert(block *KeyBlock, rec *tmprec, height int, dirty *dirty.DirtyBlocks) (*KeyBlock, *tmprec, bool) {
    r := func() (*Record) {
        if block.Mode() == self.external.Mode {
            return rec.external()
        }
        return rec.internal()
    }()
    var nextb *KeyBlock
    if height == 0 {
        // external node
        if block.Full() {
            // block is full we will need to split the block!
            // in this split case we will need to allocate another external node
        } else {
            // normal insert
        }
    }

    if height > 0 {
        // internal node
        // first we will need to find the next block to traverse down to
        // after we have found the position we get the block
        // then make a recursive call to insert to insert the record into the next block
        // if the next block split we will insert the key passed up the chain.
        // and of course check to see if this block needs to split
        // if does we will split the block, in this case we will allocate another internal node
        // we also need to handle the case where the record inserted goes into the 0th bucket,
        // but is actually smaller than the key in that bucket, in this case the search key needs
        // to be updated with the new smaller value.
    }
    // this block is changed
    dirty.Insert(block)
    if i, ok := block.Add(r); ok {
        // Block isn't full record inserted, now insert pointer (if one exists)
        // return to parent saying it has nothing to do
        if block.Mode()&POINTERS == POINTERS && nextb != nil {
            if ok := block.InsertPointer(i, nextb.Position()); !ok {
                log.Exit("pointer insert failed")
            }
        } else if block.Mode()&POINTERS == 0 && nextb != nil {
            log.Exit("tried to set a pointer on a block with no pointers")
        }
        return nil, nil, false
    }
    // Block is full split the block
    return self.split(block, rec, nextb, dirty)
}

func (self *BpTree) Insert(key ByteSlice, record []ByteSlice) bool {
    dirty := dirty.New(self.info.Height() * 4)

    // package the temp rec
    rec, valid := pkg_rec(self, key, record)
    if !valid {
        return false
    }


    // insert the block if split is true then we need to split the root
    if b, r, split := self.insert(self.getblock(self.info.Root()), rec, self.info.Height()-1, dirty); split {
        // This is where the root split goes.
        fmt.Println(b, r, split)
        // first allocate a new root then insert the key record and the associated pointers
        root := self.allocate(self.internal) // the new root will always be an internal node
        dirty.Insert(root)
        if i, ok := root.Add(r.internal()); ok {
            root.InsertPointer(i, self.info.Root())
            root.InsertPointer(i+1, b.Position())
        } else {
            fmt.Println("Could not insert into empty block PANIC")
            os.Exit(2)
            return false
        }
        // don't forget to update the height of the tree and the root
        self.info.SetRoot(root.Position())
        self.info.SetHeight(self.info.Height() + 1)
    }
    // at the end of of the method sync back the dirty blocks
    dirty.Sync()
    return true
}
