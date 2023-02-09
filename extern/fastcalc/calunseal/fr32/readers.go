package fr32

import (
	"errors"
	"io"
	"math/bits"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
)

type unpadReader struct {
	src io.Reader

	left uint64
	work []byte
}

func BufSize(sz abi.PaddedPieceSize) int {
	return int(MTTresh * mtChunkCount(sz))
}

func NewUnpadReader(src io.Reader, sz abi.PaddedPieceSize) (io.Reader, error) {
	buf := make([]byte, BufSize(sz))

	return NewUnpadReaderBuf(src, sz, buf)
}

func NewUnpadReaderBuf(src io.Reader, sz abi.PaddedPieceSize, buf []byte) (io.Reader, error) {
	if err := sz.Validate(); err != nil {
		return nil, xerrors.Errorf("bad piece size: %w", err)
	}

	return &unpadReader{
		src: src,

		left: uint64(sz),
		work: buf,
	}, nil
}

func (r *unpadReader) Read(out []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}

	chunks := len(out) / 127

	outTwoPow := 1 << (63 - bits.LeadingZeros64(uint64(chunks*128)))

	if err := abi.PaddedPieceSize(outTwoPow).Validate(); err != nil {
		return 0, xerrors.Errorf("output must be of valid padded piece size: %w", err)
	}

	todo := abi.PaddedPieceSize(outTwoPow)
	if r.left < uint64(todo) {
		todo = abi.PaddedPieceSize(1 << (63 - bits.LeadingZeros64(r.left)))
	}

	r.left -= uint64(todo)

	n, err := io.ReadAtLeast(r.src, r.work[:todo], int(todo))
	if err != nil && err != io.EOF {
		return n, err
	}
	if n < int(todo) {
		return 0, xerrors.Errorf("didn't read enough: %d / %d, left %d, out %d", n, todo, r.left, len(out))
	}

	Unpad(r.work[:todo], out[:todo.Unpadded()])

	return int(todo.Unpadded()), err
}

type padWriter struct {
	work []byte
}

func GenFr32(p []byte) []byte {
	in := p
	if len(p)%127 != 0 {
		panic("length of car file must multiples of 127")
	}
	biggest := abi.UnpaddedPieceSize(len(p))
	work := make([]byte, biggest.Padded(), biggest.Padded())
	Pad(in[:int(biggest)], work)
	return work
}

func (w *padWriter) WriteCar(p []byte) error {
	in := p
	if len(p)%127 != 0 {
		return errors.New("length of car file must multiples of 127")
	}
	biggest := abi.UnpaddedPieceSize(len(p))
	if abi.PaddedPieceSize(cap(w.work)) < biggest.Padded() {
		w.work = make([]byte, 0, biggest.Padded())
	}

	Pad(in[:int(biggest)], w.work[:int(biggest.Padded())])

	in = in[biggest:]
	return nil
}
