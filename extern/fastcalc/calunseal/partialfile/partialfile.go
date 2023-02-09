package partialfile

import (
	"encoding/binary"
	rlepluslazy "github.com/filecoin-project/go-bitfield/rle"
	"github.com/filecoin-project/go-state-types/abi"
	"go.uber.org/zap/buffer"
	"golang.org/x/xerrors"
)

type PaddedByteIndex uint64

func PieceRun(offset PaddedByteIndex, size abi.PaddedPieceSize) rlepluslazy.RunIterator {
	var runs []rlepluslazy.Run
	if offset > 0 {
		runs = append(runs, rlepluslazy.Run{
			Val: false,
			Len: uint64(offset),
		})
	}

	runs = append(runs, rlepluslazy.Run{
		Val: true,
		Len: uint64(size),
	})

	return &rlepluslazy.RunSliceIterator{Runs: runs}
}

func WriteTrailer(r rlepluslazy.RunIterator) ([]byte, error) {
	w := &buffer.Buffer{}
	trailer, err := rlepluslazy.EncodeRuns(r, nil)
	if err != nil {
		return nil, xerrors.Errorf("encoding trailer: %w", err)
	}

	rb, err := w.Write(trailer)
	if err != nil {
		return nil, xerrors.Errorf("writing trailer data: %w", err)
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(trailer))); err != nil {
		return nil, xerrors.Errorf("writing trailer length: %w", err)
	}
	return w.Bytes()[:rb+4], nil
}
