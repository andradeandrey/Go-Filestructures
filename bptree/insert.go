package bptree

import "fmt"
import "os"
import "rand"
import "math"
import "log"
import . "block/byteslice"
import . "block/keyblock"
import "block/dirty"

func init() {
    if urandom, err := os.Open("/dev/urandom", os.O_RDONLY, 0666); err != nil {
        return
    } else {
        seed := make([]byte, 8)
        if _, err := urandom.Read(seed); err == nil {
            rand.Seed(int64(ByteSlice(seed).Int64()))
        }
    }
}

func (self BpTree) findlastblock(block *KeyBlock, key ByteSlice) *KeyBlock {
    p, _ := block.GetExtraPtr()
    if p.Eq(ByteSlice64(0)) {
        return block
    }
    next := self.getblock(p)
    if r, _, _, ok := next.Get(0); !ok {
        return block
    } else {
        if r.GetKey().Eq(key) {
            return self.findlastblock(next, key)
        }
    }
    return block
}

/*
   balance blocks takes two keyblocks full, and empty and balances the records between them. full must be full
   empty must be empty
*/
func (self BpTree) balance_blocks(s int, full, empty *KeyBlock) {
    n := int(full.MaxRecordCount())
    for j := n - 1; j > s; j-- {
        // move the records
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

        //move the pointers
        if p, ok := full.GetPointer(j); ok {
            empty.InsertPointer(0, p)
        }
        full.RemovePointer(j)
    }
}

func (self *BpTree) split(a *KeyBlock, rec *tmprec, nextb *KeyBlock, dirty *dirty.DirtyBlocks) (*KeyBlock, *tmprec, bool) {

    fmt.Println("Splitting", rec.internal())
    fmt.Println("Original:\n", a)

    // this closure figures out which kind of block and record we need, either external or internal
    // we make this choice based on the mode of a because b+ trees have the property that the
    // allocated block always has the same node as the block to be split.
    b, r := func() (*KeyBlock, *Record) {
        if a.Mode() == self.external.Mode {
            return self.allocate(self.external), rec.external()
        }
        return self.allocate(self.internal), rec.internal()
    }()
    dirty.Insert(b)

    // m is the record to take as the "mid" point it is no longer stictly the mid point because
    //      one must be careful to not split up duplicate keys. instead m is the closest point to
    //      the mid point which is on the edge of the run of duplicate keys
    //
    // s is the point which the blocks should be balanced against. (ie the balance point)
    //
    // choice is the block to insert into, true is a, false is b
    m, s, choice := func() (m int, s int, choice bool) {
                getk := func(i int) ByteSlice {
                    r, _, _, _ := a.Get(i)
                    return r.GetKey()
                }
                n := int(a.MaxRecordCount()) + 1
                key := getk(n >> 1)
                l, _, _, _, _ := a.Find(key) // l = left the left side of the run of dup keys
                r := l                       // r = right the right side of the run of dup keys
                for ; r < n-1 && getk(r).Eq(key); r++ { }
                r--
                lr := math.Fabs(float64(l)/float64(n)) // left ratio (ie. how close is left to mid)
                rr := math.Fabs(float64(r)/float64(n)) // right ration (ie. how close is right to mid)
                // we return which ever has a ratio closer to zero
                if lr <= rr && l != 0{
                    m = l
                    s = l - 1 // since it is the left one we *must* subtract one from the balance point
                    choice = false
                } else {
                    m = r
                    s = r
                    choice = true
                }

                // we can only make a random choice about which block to insert the new record into
                // if there are no duplicate keys in the block, and the block fits and even number
                // of keys
//                 if (n-1)%2 == 0 && a.Count(rec.key) == 0 {
//                     f := rand.Float()
//                     if f > .5 { choice = true } else { choice = false }
//                 }
                return
            }()

    // This dependent block finds which record or split the block on.
    // it also identifies which the record should point at
    var split_rec *Record
    var nextp ByteSlice
    {
        i, _, _, _, ok := a.Find(r.GetKey())
        i--
        fmt.Printf("m=%v, s=%v, i=%v, choice=%v\n", m, s, i, choice)

        // so what is going on here is if the key is in a certain block we need to make sure
        // we insert our next key in that block. This is the cleanest way to do that for now. even
        // though it is hideously ugly.
        if ok && m > i {
//             fmt.Println("choosing a", i, m, ok)
            choice = true
        } else {
//             fmt.Println("choosing b", i, m, ok)
            choice = false
        }

        if m > i {
            // the mid point is after the spot where we would insert the key so we take the record
            // just before the mid point as our new record would shift the mid point over by 1
            split_rec, nextp, _, _ = a.Get(m - 1)
            a.RemoveAtIndex(m - 1)
            a.RemovePointer(m - 1)
        } else if m < i {
            // the mid point is before the new record so we can take the record at the mid point as
            // the split record.
            split_rec, nextp, _, _ = a.Get(m)
            a.RemoveAtIndex(m)
            a.RemovePointer(m)
        }

        if i == m {
            // the mid point is where the new record would go so in this case the new record will be
            // the split record for this split, and the nextb (which is associate with our new key)
            // become the nextp pointer
            split_rec = r
            if nextb != nil { nextp = nextb.Position() }
        } else {
            // otherwise we now need to insert our new record into the block before it is balanced
            if i, ok := a.Add(r); !ok {
                log.Exit("Inserting record into block failed PANIC")
            } else {
                // after it is inserted we need to associate the next block with its key (which is
                // the key we just inserted).
                if nextb != nil {
                    a.InsertPointer(i, nextb.Position())
                    nextb = nil
                }
            }
        }
    }
    self.balance_blocks(s, a, b)    // using s as the balance point
    fmt.Println("AFTER BALANCE")
    fmt.Println("Full:\n", a)
    fmt.Println("Empty:\n", b)

    var return_rec *Record = split_rec
    var block *KeyBlock

    // choose which block to insert the record into
    if choice {
//         fmt.Println("chose a")
        block = a
        if rec, _, _, ok := b.Get(0); !ok {
            log.Exit("Could not get the first record from block b PANIC")
        } else {
            // we change the record returned because the first record in block b will now not
            // be the record we split on which is ok we just need to return the correct record.
            return_rec = rec
        }
    } else {
//         fmt.Println("chose b")
        block = b
    }

    // add the record to the block
    if i, ok := block.Add(split_rec); !ok {
        fmt.Println(m, s, choice)
        fmt.Println("Full:\n", a)
        fmt.Println("Empty:\n", b)
        fmt.Println("Rec:", r)
        fmt.Println("return rec:", return_rec)
        log.Exit("Could not add the split_rec to the selected block PANIC")
    } else {
        // we now insert the pointer if we have one
        if block.Mode()&POINTERS == POINTERS && nextp != nil {
            // we have a block that supports a pointer and a pointer
            if !block.InsertPointer(i, nextp) {
                log.Exit("166 pointer insert failed! PANIC")
            }
        } else if block.Mode()&POINTERS == 0 && nextp != nil {
            // we don't have a block that supports a pointer but we do have a pointer
            log.Exit("tried to set a pointer on a block with no pointers")
        } else if block.Mode()&POINTERS == POINTERS && nextp == nil {
            // we have a block that supports a pointer but don't have a pointer
            log.Exit("splitting an internal block split requires a next block to point at")
        } // else
        //    we have a block that doesn't support a pointer and we don't have pointer
    }

    // if we have an external node (leaf) then we need to hook up the pointers between the leaf
    // nodes to support range queries and duplicate keys
    if a.Mode() == self.external.Mode {
        tmp, _ := a.GetExtraPtr()
        b.SetExtraPtr(tmp)
        a.SetExtraPtr(b.Position())
    }

    fmt.Println("\n\n\nAFTER EVERYTHING")
    fmt.Println("Full:\n", a)
    fmt.Println("Empty:\n", b)
    fmt.Println("Rec:", r)
    fmt.Println("return rec:", return_rec)

    return b, rec_to_tmp(self, return_rec), true
}

func (self *BpTree) insert(block *KeyBlock, rec *tmprec, height int, dirty *dirty.DirtyBlocks) (*KeyBlock, *tmprec, bool) {
    // function to take a tmprec and turn it into the appropriate type of *Record
    _convert := func(rec *tmprec) *Record {
        if block.Mode() == self.external.Mode {
            return rec.external()
        }
        return rec.internal()
    }
    r := _convert(rec)

    fmt.Println("INSERTING ", r, block, "\n\n\n")

    // nextb is the block to be passed up the the caller in the case of a split at this level.
    var nextb *KeyBlock

    if height > 0 {
        // internal node
        // first we will need to find the next block to traverse down to
        var pos ByteSlice
        {
            // we find where in the block this key would be inserted
            i, _, _, _, ok := block.Find(rec.key)

            if i == 0 {
                // if that spot is zero it means that it is less than the smallest key the block
                // so we adjust the block appropriately
                if r, p, _, ok := block.Get(i); ok {
                    dirty.Insert(block)
                    r.SetKey(rec.key)
                    pos = p
                } else {
                    log.Exitf("227 Error could not get record %v from block %v", i, block)
                }
            } else if ok {
                if _, p, _, ok := block.Get(i); ok {
                    pos = p
                } else {
                    log.Exitf("235 Error could not get record %v from block %v", i, block)
                }
            } else {
                // else this spot is one to many so we get the previous spot
                i--
                if _, p, _, ok := block.Get(i); ok {
                    pos = p
                } else {
                    log.Exitf("235 Error could not get record %v from block %v", i, block)
                }
            }
        }

        // if pos is nil we have a serious problem
        if pos == nil {
            log.Exit("242 Nil Pointer")
        }

        // after we have found the position we get the block
        // then make a recursive call to insert to insert the record into the next block
        if b, srec, s := self.insert(self.getblock(pos), rec, height-1, dirty); s {
            // if the next block split we will insert the key passed up the chain.
            nextb = b
            r = _convert(srec)
            rec = srec
        } else {
            return nil, nil, false
        }
    } else {
//         c := block.Count(rec.key)
//         ratio := float(c) / float(block.MaxRecordCount())
        if block.Full() {
            firstr, _, _, _ := block.Get(0)
            if block.Count(firstr.GetKey()) == int(block.MaxRecordCount()) {
                block := self.findlastblock(block, firstr.GetKey())
                dirty.Insert(block)
                fmt.Println("Magic Heres Abouts", r, "\n", block)
                if block.Full() || !r.GetKey().Eq(firstr.GetKey()) {
                    newblock := self.allocate(self.external)
                    dirty.Insert(newblock)
                    p, _ := block.GetExtraPtr()
                    newblock.SetExtraPtr(p)
                    block.SetExtraPtr(newblock.Position())
                    newblock.Add(r)
                    if r.GetKey().Eq(firstr.GetKey()) {
                        return nil, nil, false
                    } else {
//                         return newblock, rec_to_tmp(self, self.internal.NewRecord(firstr.GetKey().Inc())), true
                        return newblock, rec_to_tmp(self, r), true
                    }
                } else {
                    fmt.Println("adding r")
                    block.Add(r)
                    return nil, nil, false
                }
            }
        }
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
        fmt.Println("key or record not valid")
        return false
    }

    // insert the block if split is true then we need to split the root
    if b, r, split := self.insert(self.getblock(self.info.Root()), rec, self.info.Height()-1, dirty); split {
        // This is where the root split goes.

        // we have to sync the blocks back because the first key in the root will have been
        // modified if the key we inserted was less than any key in the b+ tree
        dirty.Sync()

        // we get the oldroot so we can get the first key from it, this key becomes the first key in
        // the new root.
        oldroot := self.getblock(self.info.Root())
        var first *tmprec
        if f, _, _, ok := oldroot.Get(0); ok {
            first = rec_to_tmp(self, f)
        }

        // first allocate a new root then insert the key record and the associated pointers
        root := self.allocate(self.internal) // the new root will always be an internal node
        dirty.Insert(root)

        // first we insert the first key from the old root into the new root and point it at the
        // old root
        if i, ok := root.Add(first.internal()); ok {
            root.InsertPointer(i, self.info.Root())
        } else {
            fmt.Println("Could not insert into empty block PANIC")
            os.Exit(2)
            return false
        }

        // then we point the split rec's key at the the split block
        if i, ok := root.Add(r.internal()); ok {
            root.InsertPointer(i, b.Position())
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
