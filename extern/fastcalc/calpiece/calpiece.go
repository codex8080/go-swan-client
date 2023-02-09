package calpiece

import (
	"crypto/sha256"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/ipfs/go-cid"
	"runtime"
	"sync"
)

type GenPieceFactory struct {
	bTree         [][32]byte
	level         int
	baseLength    int
	baseZeroCount int
	criticality   float64
}

const NODE_SIZE = 32

func NewGenPieceFactory(pieceSize int, fr32data []byte, criticality float64) (*GenPieceFactory, error) {
	baseNodeN := pieceSize / NODE_SIZE
	sumN := baseNodeN*2 - 1
	level := pow2(baseNodeN)
	fr32Size := len(fr32data)
	if fr32Size%32 != 0 {
		panic("length of fr32data must multiples of 32")
	}
	realNodeN := fr32Size / NODE_SIZE
	bTree := make([][32]byte, sumN, sumN)
	begin := 1<<level - 1
	for i := 0; i < realNodeN; i++ {
		copy(bTree[begin+i][:], fr32data[i*32:i*32+32])
	}

	p := &GenPieceFactory{
		bTree:         bTree,
		level:         level,
		baseLength:    baseNodeN,
		baseZeroCount: baseNodeN - realNodeN,
		criticality:   criticality,
	}
	return p, nil
}

func (p *GenPieceFactory) sumZero() {
	zeroNodeNum := p.baseZeroCount
	subZeroPieceEnd := p.baseLength*2 - 1
	dealZeroNum := 0
	for {
		if zeroNodeNum < 16 {
			break
		}
		subZeroPieceLength := 2
		subZeroHeight := 2
		for {
			subZeroPieceLength <<= 1
			subZeroHeight++
			if subZeroPieceLength > zeroNodeNum {
				subZeroPieceLength >>= 1
				subZeroHeight--
				break
			}
		}
		subRootIndex := subZeroPieceEnd - 1
		for i := 1; i < subZeroHeight; i++ {
			subRootIndex = (subRootIndex - 1) / 2
		}

		p.bTree[subRootIndex] = ZeroPieceNode[subZeroHeight]
		zeroNodeNum -= subZeroPieceLength
		subZeroPieceEnd -= subZeroPieceLength
		dealZeroNum += subZeroPieceLength
	}
	p.baseZeroCount = dealZeroNum
}

func (p *GenPieceFactory) Sum() (cid.Cid, error) {
	p.sumZero()
	cpuN := runtime.NumCPU()

	levelOffset := 0
	var parallelN = 1
	for cpuN > 1 {
		if levelOffset+3 >= p.level {
			break
		}
		cpuN = cpuN >> 1
		parallelN = parallelN << 1
		levelOffset++
	}

	base := parallelN
	zeroIndex := p.baseLength*2 - 1 - p.baseZeroCount

	var offsetZeroIndex int
	var shouldParallelN int
	subPieceLength := p.baseLength / parallelN
	if (p.baseZeroCount)%subPieceLength != 0 {
		shouldParallelN = (p.baseLength-p.baseZeroCount)/subPieceLength + 1
		offsetZeroIndex = 1
	} else {
		shouldParallelN = (p.baseLength - p.baseZeroCount) / subPieceLength
	}

	if float64(shouldParallelN)*2/float64(parallelN) < p.criticality {
		subPieceLength := p.baseLength / parallelN / 2
		if (p.baseZeroCount)%subPieceLength != 0 {
			shouldParallelN = (p.baseLength-p.baseZeroCount)/subPieceLength + 1
		} else {
			shouldParallelN = (p.baseLength - p.baseZeroCount) / subPieceLength
		}
		levelOffset++
		base *= 2

	}

	wg := sync.WaitGroup{}
	wg.Add(shouldParallelN)
	for i := 0; i < shouldParallelN; i++ {
		var sum Sum
		if i+1 == shouldParallelN {
			sum = NewSubPiecePaddZero(p.bTree, p.level, i, base, zeroIndex)
		} else {
			sum = NewSubPiece(p.bTree, p.level, i, base)
		}
		go func() {
			defer wg.Done()
			sum.Sum()
		}()
	}
	wg.Wait()

	for i := 0; i < p.level-levelOffset; i++ {
		zeroIndex = (zeroIndex - 1) >> 1
	}
	end := zeroIndex + offsetZeroIndex
	p.level = levelOffset

	hash := sha256.New()
	for p.level > 0 {
		cur := 1<<p.level - 1
		for cur < end {

			hash.Reset()
			hash.Write(p.bTree[cur][:])
			hash.Write(p.bTree[cur+1][:])
			bytes := hash.Sum(nil)
			bytes[31] &= 0b00111111
			copy(p.bTree[(cur-1)/2][:], bytes)
			cur += 2
		}
		if cur == end {
			end = (end - 1) / 2
		} else {
			end /= 2
		}
		p.level--
	}

	return commcid.PieceCommitmentV1ToCID(p.bTree[0][:])
}

func (p *GenPieceFactory) Close() {
	p.bTree = nil
}

func pow2(n int) int {
	res := 0
	for n > 1 {
		n = n >> 1
		res++
	}
	return res
}

type SubPiecePaddZero struct {
	pTree [][32]byte
	level int
	index int
	base  int
	end   int
}

func NewSubPiecePaddZero(pTree [][32]byte, level int, index int, base int, end int) Sum {
	return &SubPiecePaddZero{
		pTree: pTree,
		level: level,
		index: index,
		base:  base,
		end:   end,
	}
}

func (sp *SubPiecePaddZero) Sum() {
	levelOffset := pow2(sp.base)
	hash := sha256.New()
	baseEnd := sp.end
	for sp.level > levelOffset {
		length := 1 << sp.level
		begin := length - 1
		baseLength := length / sp.base
		cur := begin + (baseLength * sp.index)
		for cur < baseEnd {
			hash.Reset()
			hash.Write(sp.pTree[cur][:])
			hash.Write(sp.pTree[cur+1][:])
			bytes := hash.Sum(nil)
			bytes[31] &= 0b00111111
			copy(sp.pTree[(cur-1)/2][:], bytes)
			cur += 2
		}
		if cur == baseEnd {
			baseEnd = (baseEnd - 1) / 2
		} else {
			baseEnd /= 2
		}
		sp.level--
	}
}

type SubPiece struct {
	pTree [][32]byte
	level int
	index int
	base  int
}

type Sum interface {
	Sum()
}

func NewSubPiece(pTree [][32]byte, level int, index int, base int) Sum {
	return &SubPiece{
		pTree: pTree,
		level: level,
		index: index,
		base:  base,
	}
}

func (sp *SubPiece) Sum() {
	levelOffset := pow2(sp.base)
	hash := sha256.New()

	for sp.level > levelOffset {
		length := 1 << sp.level
		begin := length - 1
		baseLength := length / sp.base
		cur := begin + (baseLength * sp.index)
		baseEnd := cur + baseLength
		for cur < baseEnd {
			hash.Reset()
			hash.Write(sp.pTree[cur][:])
			hash.Write(sp.pTree[cur+1][:])
			bytes := hash.Sum(nil)
			bytes[31] &= 0b00111111
			copy(sp.pTree[(cur-1)/2][:], bytes)
			cur += 2
		}
		sp.level--
	}
}

var ZeroPieceNode = [34][32]byte{
	0:  {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	1:  {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	2:  {245, 165, 253, 66, 209, 106, 32, 48, 39, 152, 239, 110, 211, 9, 151, 155, 67, 0, 61, 35, 32, 217, 240, 232, 234, 152, 49, 169, 39, 89, 251, 11},
	3:  {55, 49, 187, 153, 172, 104, 159, 102, 238, 245, 151, 62, 74, 148, 218, 24, 143, 77, 220, 174, 88, 7, 36, 252, 111, 63, 214, 13, 253, 72, 131, 51},
	4:  {100, 42, 96, 126, 248, 134, 176, 4, 191, 44, 25, 120, 70, 58, 225, 212, 105, 58, 192, 244, 16, 235, 45, 27, 122, 71, 254, 32, 94, 94, 117, 15},
	5:  {87, 162, 56, 26, 40, 101, 43, 244, 127, 107, 239, 122, 202, 103, 155, 228, 174, 222, 88, 113, 171, 92, 243, 235, 44, 8, 17, 68, 136, 203, 133, 38},
	6:  {31, 122, 201, 89, 85, 16, 224, 158, 164, 28, 70, 11, 23, 100, 48, 187, 50, 44, 214, 251, 65, 46, 197, 124, 177, 125, 152, 154, 67, 16, 55, 47},
	7:  {252, 126, 146, 130, 150, 229, 22, 250, 173, 233, 134, 178, 143, 146, 212, 74, 79, 36, 185, 53, 72, 82, 35, 55, 106, 121, 144, 39, 188, 24, 248, 51},
	8:  {8, 196, 123, 56, 238, 19, 188, 67, 244, 27, 145, 92, 14, 237, 153, 17, 162, 96, 134, 179, 237, 98, 64, 27, 249, 213, 139, 141, 25, 223, 246, 36},
	9:  {178, 228, 123, 251, 17, 250, 205, 148, 31, 98, 175, 92, 117, 15, 62, 165, 204, 77, 245, 23, 213, 196, 241, 109, 178, 180, 215, 123, 174, 193, 163, 47},
	10: {249, 34, 97, 96, 200, 249, 39, 191, 220, 196, 24, 205, 242, 3, 73, 49, 70, 0, 142, 174, 251, 125, 2, 25, 77, 94, 84, 129, 137, 0, 81, 8},
	11: {44, 26, 150, 75, 185, 11, 89, 235, 254, 15, 109, 162, 154, 214, 90, 227, 228, 23, 114, 74, 143, 124, 17, 116, 90, 64, 202, 193, 229, 231, 64, 17},
	12: {254, 227, 120, 206, 241, 100, 4, 177, 153, 237, 224, 177, 62, 17, 182, 36, 255, 157, 120, 79, 187, 237, 135, 141, 131, 41, 126, 121, 94, 2, 79, 2},
	13: {142, 158, 36, 3, 250, 136, 76, 246, 35, 127, 96, 223, 37, 248, 62, 228, 13, 202, 158, 216, 121, 235, 111, 99, 82, 209, 80, 132, 245, 173, 13, 63},
	14: {117, 45, 150, 147, 250, 22, 117, 36, 57, 84, 118, 227, 23, 169, 133, 128, 240, 9, 71, 175, 183, 163, 5, 64, 214, 37, 169, 41, 28, 193, 42, 7},
	15: {112, 34, 246, 15, 126, 246, 173, 250, 23, 17, 122, 82, 97, 158, 48, 206, 168, 44, 104, 7, 90, 223, 28, 102, 119, 134, 236, 80, 110, 239, 45, 25},
	16: {217, 152, 135, 185, 115, 87, 58, 150, 225, 19, 147, 100, 82, 54, 193, 123, 31, 76, 112, 52, 215, 35, 199, 169, 159, 112, 155, 180, 218, 97, 22, 43},
	17: {208, 181, 48, 219, 176, 180, 242, 92, 93, 47, 42, 40, 223, 238, 128, 139, 83, 65, 42, 2, 147, 31, 24, 196, 153, 245, 162, 84, 8, 107, 19, 38},
	18: {132, 192, 66, 27, 160, 104, 90, 1, 191, 121, 90, 35, 68, 6, 79, 228, 36, 189, 82, 169, 210, 67, 119, 179, 148, 255, 76, 75, 69, 104, 232, 17},
	19: {101, 242, 158, 93, 152, 210, 70, 195, 139, 56, 140, 252, 6, 219, 31, 107, 2, 19, 3, 197, 162, 137, 0, 11, 220, 232, 50, 169, 195, 236, 66, 28},
	20: {162, 36, 117, 8, 40, 88, 80, 150, 91, 126, 51, 75, 49, 39, 176, 192, 66, 177, 208, 70, 220, 84, 64, 33, 55, 98, 124, 216, 121, 156, 225, 58},
	21: {218, 253, 171, 109, 169, 54, 68, 83, 194, 109, 51, 114, 107, 159, 239, 227, 67, 190, 143, 129, 100, 158, 192, 9, 170, 211, 250, 255, 80, 97, 117, 8},
	22: {217, 65, 213, 224, 214, 49, 74, 153, 92, 51, 255, 189, 79, 190, 105, 17, 141, 115, 212, 229, 253, 44, 211, 31, 15, 124, 134, 235, 221, 20, 231, 6},
	23: {81, 76, 67, 92, 61, 4, 211, 73, 165, 54, 95, 189, 89, 255, 199, 19, 98, 145, 17, 120, 89, 145, 193, 163, 197, 58, 242, 32, 121, 116, 26, 47},
	24: {173, 6, 133, 57, 105, 211, 125, 52, 255, 8, 224, 159, 86, 147, 10, 74, 209, 154, 137, 222, 246, 12, 191, 238, 126, 29, 51, 129, 193, 231, 28, 55},
	25: {57, 86, 14, 123, 19, 169, 59, 7, 162, 67, 253, 39, 32, 255, 167, 203, 62, 29, 46, 80, 90, 179, 98, 158, 121, 244, 99, 19, 81, 44, 218, 6},
	26: {204, 195, 192, 18, 245, 176, 94, 129, 26, 43, 191, 221, 15, 104, 51, 184, 66, 117, 180, 123, 242, 41, 192, 5, 42, 130, 72, 79, 60, 26, 91, 61},
	27: {125, 242, 155, 105, 119, 49, 153, 232, 242, 180, 11, 119, 145, 157, 4, 133, 9, 238, 215, 104, 226, 199, 41, 123, 31, 20, 55, 3, 79, 195, 198, 44},
	28: {102, 206, 5, 163, 102, 117, 82, 207, 69, 192, 43, 204, 78, 131, 146, 145, 155, 222, 172, 53, 222, 47, 245, 98, 113, 132, 142, 159, 123, 103, 81, 7},
	29: {216, 97, 2, 24, 66, 90, 181, 233, 91, 28, 166, 35, 157, 41, 162, 228, 32, 215, 6, 169, 111, 55, 62, 47, 156, 154, 145, 215, 89, 209, 155, 1},
	30: {109, 54, 75, 30, 248, 70, 68, 26, 90, 74, 104, 134, 35, 20, 172, 192, 164, 111, 1, 103, 23, 229, 52, 67, 232, 57, 238, 223, 131, 194, 133, 60},
	31: {7, 126, 95, 222, 53, 197, 10, 147, 3, 165, 80, 9, 227, 73, 138, 78, 190, 223, 243, 156, 66, 183, 16, 183, 48, 216, 236, 122, 199, 175, 166, 62},
	32: {230, 64, 5, 166, 191, 227, 119, 121, 83, 184, 173, 110, 249, 63, 15, 202, 16, 73, 178, 4, 22, 84, 242, 164, 17, 247, 112, 39, 153, 206, 206, 2},
	33: {37, 157, 61, 107, 31, 77, 135, 109, 17, 133, 225, 18, 58, 246, 245, 80, 26, 240, 246, 124, 241, 91, 82, 22, 37, 91, 123, 23, 141, 18, 5, 29},
}
