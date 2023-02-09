package fastcalc

import (
	"fmt"
	padread "github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filswan/go-swan-client/extern/fastcalc/calpiece"
	"github.com/filswan/go-swan-client/extern/fastcalc/calunseal"
	"github.com/ipfs/go-cid"
	"io"
	"os"
)

func FastCommP(carFileName string) (pieceCid cid.Cid, unPadSize abi.UnpaddedPieceSize, err error) {
	carFile, err := os.Open(carFileName)
	if err != nil {
		return pieceCid, unPadSize, err
	}
	defer carFile.Close()

	stat, err := carFile.Stat()
	if err != nil {
		return pieceCid, unPadSize, err
	}

	carlData, err := io.ReadAll(carFile)
	if err != nil {
		return pieceCid, unPadSize, err
	}

	unPadSize = padread.PaddedSize(uint64(stat.Size()))
	padPieceSize, unsealData, err := calunseal.NewUnsealData(abi.PaddedPieceSize(32<<30), carlData)
	if err != nil {
		return pieceCid, unPadSize, err
	}
	genFactory, err := calpiece.NewGenPieceFactory(int(padPieceSize), unsealData.Fr32Data, 1.2)
	if err != nil {
		return pieceCid, unPadSize, err
	}
	defer genFactory.Close()

	pieceCid, err = genFactory.Sum()
	if err != nil {
		return pieceCid, unPadSize, err
	}
	fmt.Println("pieceCid:", pieceCid, " pieceSize:", unPadSize)

	return pieceCid, unPadSize, nil
}
