package calunseal

import (
	"github.com/filecoin-project/go-state-types/abi"
	fr322 "github.com/filswan/go-swan-client/extern/fastcalc/calunseal/fr32"
	"github.com/filswan/go-swan-client/extern/fastcalc/calunseal/partialfile"
	"io"
)

type UnsealData struct {
	size     int
	Fr32Data []byte
	trailer  []byte
}

func NewUnsealData(size abi.PaddedPieceSize, car []byte) (abi.PaddedPieceSize, *UnsealData, error) {
	if len(car)%127 != 0 {
		pad := make([]byte, 127-len(car)%127)
		car = append(car, pad...)
	}

	fr32File := fr322.GenFr32(car)
	ored := partialfile.PieceRun(0, size)
	trailer, err := partialfile.WriteTrailer(ored)
	if err != nil {
		return 0, nil, err
	}
	uf := &UnsealData{
		size:     int(size),
		Fr32Data: fr32File,
		trailer:  trailer,
	}
	return size, uf, nil
}

func (ud *UnsealData) Reader() *UnsealReader {
	return &UnsealReader{
		*ud,
		0,
	}
}

func (ud *UnsealData) Close() {
	ud.Fr32Data = nil
	ud.trailer = nil
}

type UnsealReader struct {
	UnsealData
	off int
}

func (uf *UnsealReader) Read(p []byte) (n int, err error) {
	if uf.off == uf.size+len(uf.trailer) {
		// Buffer is empty, reset to recover space.
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n0 := 0
	if uf.off < len(uf.Fr32Data) {
		n0 = copy(p, uf.Fr32Data[uf.off:])
		uf.off += n0
	}
	n += n0
	if len(p) == n0 {
		return n, nil
	}

	p = p[n0:]
	n1 := 0
	for i := range p {
		if uf.off < uf.size {
			p[i] = 0
			uf.off++
			n1++
		} else {
			break
		}
	}
	n += n1
	if len(p) == n1 {
		return n, nil
	}
	p = p[n1:]
	allSize := uf.size + len(uf.trailer)
	n2 := 0
	if uf.off < allSize {
		n2 = copy(p, uf.trailer[uf.off-uf.size:])
		uf.off += n2
	}
	n += n2
	return n, nil
}

func (uf *UnsealReader) GetFr32Data() []byte {
	return uf.Fr32Data
}

func (uf *UnsealReader) Close() {
	uf.Fr32Data = nil
	uf.trailer = nil
}
