// Copyright 2017 Factom Foundation
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package systemState

import (
	"fmt"

	"github.com/FactomProject/factomd/blockchainState"
	"github.com/FactomProject/factomd/blockchainState/blockMaker"
	"github.com/FactomProject/factomd/common/constants"
	"github.com/FactomProject/factomd/common/interfaces"
	"github.com/FactomProject/factomd/common/messages"
)

type BStateHandler struct {
	//Main, full BState
	MainBState *blockchainState.BlockchainState
	//BState for synching from the Genesis Block
	//BacklogBState *blockchainState.BlockchainState
	//BlockMaker for making the next set of blocks
	BlockMaker *blockMaker.BlockMaker
	//Database for storing new blocks and entries
	DB interfaces.DBOverlay

	//DBStateMsgs that have not been applied or dismissed yet
	PendingDBStateMsgs []*messages.DBStateMsg

	//Marking whether we're still synchronising with the network, or are we fully synched
	//FullySynched bool
}

func (bh *BStateHandler) InitMainNet() {
	if bh.MainBState == nil {
		bh.MainBState = blockchainState.NewBSMainNet()
	}
}

func (bh *BStateHandler) InitTestNet() {
	if bh.MainBState == nil {
		bh.MainBState = blockchainState.NewBSTestNet()
	}
}

func (bh *BStateHandler) InitLocalNet() {
	if bh.MainBState == nil {
		bh.MainBState = blockchainState.NewBSLocalNet()
	}
}

func (bh *BStateHandler) LoadDatabase() error {
	if bh.DB == nil {
		return fmt.Errorf("No DB present")
	}

	err := bh.LoadBState()
	if err != nil {
		return err
	}

	start := 0
	if bh.MainBState.DBlockHeight > 0 {
		start = int(bh.MainBState.DBlockHeight) + 1
	}

	dbHead, err := bh.DB.FetchDBlockHead()
	if err != nil {
		return err
	}
	end := 0
	if dbHead != nil {
		end = int(dbHead.GetDatabaseHeight())
	} else {
		//database is empty, initialise it
		//TODO: do
	}

	for i := start; i < end; i++ {
		set := FetchBlockSet(bh.DB, i)

		err := bh.MainBState.ProcessBlockSet(set.DBlock, set.ABlock, set.FBlock, set.ECBlock, set.EBlocks, set.Entries)
		if err != nil {
			return err
		}

		//TODO: save BState periodically
	}

	err = bh.SaveBState(bh.MainBState)
	if err != nil {
		return err
	}

	return nil
}

func (bh *BStateHandler) HandleDBStateMsg(msg interfaces.IMsg) error {
	if msg.Type() != constants.DBSTATE_MSG {
		return fmt.Errorf("Invalid message type")
	}
	dbStateMsg := msg.(*messages.DBStateMsg)

	height := dbStateMsg.DirectoryBlock.GetDatabaseHeight()
	if bh.MainBState.DBlockHeight >= height {
		if height != 0 {
			//Nothing to do - we're already ahead
			return nil
		}
		if !bh.MainBState.DBlockHead.KeyMR.IsZero() {
			//Nothing to do - we're already ahead
			return nil
		}
		//We're processing genesis block!
	}
	if bh.MainBState.DBlockHeight+1 < height {
		//DBStateMsg is too far ahead - ignore it for now
		bh.PendingDBStateMsgs = append(bh.PendingDBStateMsgs, dbStateMsg)
		return nil
	}

	tmpBState, err := bh.MainBState.Clone()
	if err != nil {
		return err
	}

	err = tmpBState.ProcessBlockSet(dbStateMsg.DirectoryBlock, dbStateMsg.AdminBlock, dbStateMsg.FactoidBlock, dbStateMsg.EntryCreditBlock,
		dbStateMsg.EBlocks, dbStateMsg.Entries)
	if err != nil {
		return err
	}

	err = bh.SaveBlockSetToDB(dbStateMsg.DirectoryBlock, dbStateMsg.AdminBlock, dbStateMsg.FactoidBlock, dbStateMsg.EntryCreditBlock,
		dbStateMsg.EBlocks, dbStateMsg.Entries)
	if err != nil {
		return err
	}

	bh.MainBState = tmpBState

	err = bh.SaveBState(bh.MainBState)
	if err != nil {
		return err
	}

	for i := len(bh.PendingDBStateMsgs) - 1; i >= 0; i-- {
		if bh.PendingDBStateMsgs[i].DirectoryBlock.GetDatabaseHeight() <= bh.MainBState.DBlockHeight {
			//We already dealt with this DBState, deleting the message
			bh.PendingDBStateMsgs = append(bh.PendingDBStateMsgs[:i], bh.PendingDBStateMsgs[i+1:]...)
		}
		if bh.PendingDBStateMsgs[i].DirectoryBlock.GetDatabaseHeight() == bh.MainBState.DBlockHeight+1 {
			//Next DBState to process - do it now
			err = bh.HandleDBStateMsg(bh.PendingDBStateMsgs[i])
			if err != nil {
				return err
			}
		}
	}

	//TODO: overwrite BlockMaker if appropriate
	s, err := bh.MainBState.Clone()
	if err != nil {
		return nil
	}
	bh.BlockMaker = blockMaker.NewBlockMaker()
	bh.BlockMaker.BState = s

	return nil
}

func (bh *BStateHandler) SaveBState(bState *blockchainState.BlockchainState) error {
	//TODO: figure out how often we want to save BStates
	//TODO: do
	return nil
}

func (bh *BStateHandler) LoadBState() error {
	//TODO: do
	return nil
}

func (bh *BStateHandler) SaveBlockSetToDB(dBlock interfaces.IDirectoryBlock, aBlock interfaces.IAdminBlock, fBlock interfaces.IFBlock,
	ecBlock interfaces.IEntryCreditBlock, eBlocks []interfaces.IEntryBlock, entries []interfaces.IEBEntry) error {

	bh.DB.StartMultiBatch()

	err := bh.DB.ProcessDBlockMultiBatch(dBlock)
	if err != nil {
		bh.DB.CancelMultiBatch()
		return err
	}
	err = bh.DB.ProcessABlockMultiBatch(aBlock)
	if err != nil {
		bh.DB.CancelMultiBatch()
		return err
	}
	err = bh.DB.ProcessFBlockMultiBatch(fBlock)
	if err != nil {
		bh.DB.CancelMultiBatch()
		return err
	}
	err = bh.DB.ProcessECBlockMultiBatch(ecBlock, false)
	if err != nil {
		bh.DB.CancelMultiBatch()
		return err
	}
	for _, e := range eBlocks {
		err = bh.DB.ProcessEBlockMultiBatch(e, false)
		if err != nil {
			return err
		}
	}
	for _, e := range entries {
		err = bh.DB.InsertEntryMultiBatch(e)
		if err != nil {
			bh.DB.CancelMultiBatch()
			return err
		}
	}

	err = bh.DB.ExecuteMultiBatch()
	if err != nil {
		return err
	}

	return nil
}

func (bs *BStateHandler) ProcessAckedMessage(msg interfaces.IMessageWithEntry, ack *messages.Ack) error {
	return bs.BlockMaker.ProcessAckedMessage(msg, ack)
}

type BlockSet struct {
	ABlock  interfaces.IAdminBlock
	ECBlock interfaces.IEntryCreditBlock
	FBlock  interfaces.IFBlock
	DBlock  interfaces.IDirectoryBlock
	EBlocks []interfaces.IEntryBlock
	Entries []interfaces.IEBEntry
}

func FetchBlockSet(dbo interfaces.DBOverlay, index int) *BlockSet {
	bs := new(BlockSet)

	dBlock, err := dbo.FetchDBlockByHeight(uint32(index))
	if err != nil {
		panic(err)
	}
	bs.DBlock = dBlock

	if dBlock == nil {
		return bs
	}
	entries := dBlock.GetDBEntries()
	for _, entry := range entries {
		switch entry.GetChainID().String() {
		case "000000000000000000000000000000000000000000000000000000000000000a":
			aBlock, err := dbo.FetchABlock(entry.GetKeyMR())
			if err != nil {
				panic(err)
			}
			bs.ABlock = aBlock
			break
		case "000000000000000000000000000000000000000000000000000000000000000c":
			ecBlock, err := dbo.FetchECBlock(entry.GetKeyMR())
			if err != nil {
				panic(err)
			}
			bs.ECBlock = ecBlock
			break
		case "000000000000000000000000000000000000000000000000000000000000000f":
			fBlock, err := dbo.FetchFBlock(entry.GetKeyMR())
			if err != nil {
				panic(err)
			}
			bs.FBlock = fBlock
			break
		default:
			eBlock, err := dbo.FetchEBlock(entry.GetKeyMR())
			if err != nil {
				panic(err)
			}
			bs.EBlocks = append(bs.EBlocks, eBlock)

			//Fetching special entries
			if blockchainState.IsSpecialBlock(eBlock.GetChainID()) {
				for _, v := range eBlock.GetEntryHashes() {
					if v.IsMinuteMarker() {
						continue
					}
					e, err := dbo.FetchEntry(v)
					if err != nil {
						panic(err)
					}
					if e == nil {
						panic("Couldn't find entry " + v.String())
					}
					bs.Entries = append(bs.Entries, e)
				}
			}

			/*
				for _, v := range eBlock.GetEntryHashes() {
					if v.IsMinuteMarker() {
						continue
					}
					e, err := dbo.FetchEntry(v)
					if err != nil {
						panic(err)
					}
					if e == nil {
						panic("Couldn't find entry " + v.String())
					}
					bs.Entries = append(bs.Entries, e)
				}
			*/
			break
		}
	}

	return bs
}