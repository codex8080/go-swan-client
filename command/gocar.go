package command

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/filswan/go-swan-lib/client/lotus"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/filswan/go-swan-client/config"
	"github.com/filswan/go-swan-client/extern/fastcalc"
	"github.com/google/uuid"

	"github.com/filswan/go-swan-lib/logs"

	"github.com/filswan/go-swan-lib/utils"

	"github.com/codingsince1985/checksum"
	"github.com/filedrive-team/go-graphsplit"
	libmodel "github.com/filswan/go-swan-lib/model"
	ipld "github.com/ipfs/go-ipld-format"
)

type CmdGoCar struct {
	LotusClientApiUrl      string //required
	LotusClientAccessToken string //required
	OutputDir              string //required
	InputDir               string //required
	GenerateMd5            bool   //required
	GocarFileSizeLimit     int64  //required
	GocarFolderBased       bool   //required
	Parallel               int
	ImportFlag             bool
}

func GetCmdGoCar(inputDir string, outputDir *string, parallel int, carFileSizeLimit int64, carFolderBased, importFlag bool) *CmdGoCar {
	cmdGoCar := &CmdGoCar{
		LotusClientApiUrl:      config.GetConfig().Lotus.ClientApiUrl,
		LotusClientAccessToken: config.GetConfig().Lotus.ClientAccessToken,
		InputDir:               inputDir,
		GocarFileSizeLimit:     carFileSizeLimit,
		GenerateMd5:            config.GetConfig().Sender.GenerateMd5,
		GocarFolderBased:       carFolderBased,
		Parallel:               parallel,
		ImportFlag:             importFlag,
	}

	if !utils.IsStrEmpty(outputDir) {
		cmdGoCar.OutputDir = *outputDir
	} else {
		cmdGoCar.OutputDir = filepath.Join(*outputDir, time.Now().Format("2006-01-02_15:04:05")) + "_" + uuid.NewString()
	}

	return cmdGoCar
}

func CreateGoCarFilesByConfig(inputDir string, outputDir *string, parallel int, carFileSizeLimit int64, carFolderBased, importFlag bool) ([]*libmodel.FileDesc, error) {
	cmdGoCar := GetCmdGoCar(inputDir, outputDir, parallel, carFileSizeLimit, carFolderBased, importFlag)
	fileDescs, err := cmdGoCar.CreateGoCarFiles()
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	return fileDescs, nil
}

func RestoreCarFilesByConfig(inputDir string, outputDir *string, parallel int) error {
	cmdGoCar := GetCmdGoCar(inputDir, outputDir, parallel, 0, false, false)
	err := cmdGoCar.RestoreCarToFiles()
	if err != nil {
		logs.GetLogger().Error(err)
		return err
	}
	return nil
}

type faseCommPCallback struct {
	carDir string
}

func (cc *faseCommPCallback) OnSuccess(node ipld.Node, graphName, fsDetail string) {
	commpStartTime := time.Now()
	carfilepath := path.Join(cc.carDir, node.Cid().String()+".car")

	var err error
	cpRes := &graphsplit.CommPRet{}
	carSize := utils.GetFileSize(carfilepath)
	if carSize > 17179869184 { //16G 17179869184
		logs.GetLogger().Info("###### using fast calculation of piece cid ######")
		pieceCid, unPadSize, err := fastcalc.FastCommP(carfilepath)
		if err != nil {
			logs.GetLogger().Fatal(err)
		}
		cpRes.Root = pieceCid
		cpRes.Size = unPadSize
	} else {
		cp, err := graphsplit.CalcCommP(context.TODO(), carfilepath)
		if err != nil {
			logs.GetLogger().Fatal(err)
		}
		cpRes.Root = cp.Root
		cpRes.Size = cp.Size
	}

	logs.GetLogger().Infof("calculation %s: cid=%s size=%d,", carfilepath, cpRes.Root, cpRes.Size)
	logs.GetLogger().Infof("calculation of pieceCID completed, time elapsed: %s", time.Now().Sub(commpStartTime))
	// Add node inof to manifest.csv
	manifestPath := path.Join(cc.carDir, "manifest.csv")
	_, err = os.Stat(manifestPath)
	if err != nil && !os.IsNotExist(err) {
		logs.GetLogger().Fatal(err)
	}
	var isCreateAction bool
	if err != nil && os.IsNotExist(err) {
		isCreateAction = true
	}
	f, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logs.GetLogger().Fatal(err)
	}
	defer f.Close()
	if isCreateAction {
		if _, err := f.Write([]byte("playload_cid,filename,piece_cid,piece_size,detail\n")); err != nil {
			logs.GetLogger().Fatal(err)
		}
	}
	if _, err := f.Write([]byte(fmt.Sprintf("%s,%s,%s,%d,%s\n", node.Cid(), graphName, cpRes.Root.String(), cpRes.Size, fsDetail))); err != nil {
		logs.GetLogger().Fatal(err)
	}
}

func (cc *faseCommPCallback) OnError(err error) {
	logs.GetLogger().Fatal(err)
}

func FastCommPCallback(carDir string) graphsplit.GraphBuildCallback {
	return &faseCommPCallback{carDir: carDir}
}

func (cmdGoCar *CmdGoCar) CreateGoCarFiles() ([]*libmodel.FileDesc, error) {
	err := utils.CheckDirExists(cmdGoCar.InputDir, DIR_NAME_INPUT)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	err = utils.CreateDirIfNotExists(cmdGoCar.OutputDir, DIR_NAME_OUTPUT)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	sliceSize := cmdGoCar.GocarFileSizeLimit
	if sliceSize <= 0 {
		err := fmt.Errorf("gocar file size limit is too smal")
		logs.GetLogger().Error(err)
		return nil, err
	}

	srcFiles, err := ioutil.ReadDir(cmdGoCar.InputDir)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	carDir := cmdGoCar.OutputDir
	Emptyctx := context.Background()

	cb := FastCommPCallback(carDir)

	if cmdGoCar.GocarFolderBased {
		parentPath := cmdGoCar.InputDir
		targetPath := parentPath
		graphName := filepath.Base(parentPath)

		logs.GetLogger().Info("Creating car file for ", parentPath)
		err = graphsplit.Chunk(Emptyctx, sliceSize, parentPath, targetPath, carDir, graphName, cmdGoCar.Parallel, cb)
		if err != nil {
			logs.GetLogger().Error(err)
			return nil, err
		}
		logs.GetLogger().Info("Car file for ", parentPath, " created")
	} else {
		for _, srcFile := range srcFiles {
			parentPath := filepath.Join(cmdGoCar.InputDir, srcFile.Name())
			targetPath := parentPath
			graphName := srcFile.Name()

			logs.GetLogger().Info("Creating car file for ", parentPath)
			err = graphsplit.Chunk(Emptyctx, sliceSize, parentPath, targetPath, carDir, graphName, cmdGoCar.Parallel, cb)
			if err != nil {
				logs.GetLogger().Error(err)
				return nil, err
			}
			logs.GetLogger().Info("Car file for ", parentPath, " created")
		}
	}
	fileDescs, err := cmdGoCar.createFilesDescFromManifest()
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	logs.GetLogger().Info(len(fileDescs), " car files have been created to directory:", carDir)
	logs.GetLogger().Info("Please upload car files to web server or ipfs server.")

	return fileDescs, nil
}

func (cmdGoCar *CmdGoCar) RestoreCarToFiles() error {
	err := utils.CheckDirExists(cmdGoCar.InputDir, DIR_NAME_INPUT)
	if err != nil {
		logs.GetLogger().Error(err)
		return err
	}

	err = utils.CreateDirIfNotExists(cmdGoCar.OutputDir, DIR_NAME_OUTPUT)
	if err != nil {
		logs.GetLogger().Error(err)
		return err
	}

	graphsplit.CarTo(cmdGoCar.InputDir, cmdGoCar.OutputDir, cmdGoCar.Parallel)
	graphsplit.Merge(cmdGoCar.OutputDir, cmdGoCar.Parallel)

	logs.GetLogger().Info("car files have been restored to directory:", cmdGoCar.OutputDir)
	return nil
}

type ManifestDetail struct {
	Name string
	Hash string
	Size int
	Link []struct {
		Name string
		Hash string
		Size int64
	}
}

func (cmdGoCar *CmdGoCar) createFilesDescFromManifest() ([]*libmodel.FileDesc, error) {
	manifestFilename := "manifest.csv"
	lines, err := utils.ReadAllLines(cmdGoCar.OutputDir, manifestFilename)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}
	var lotusClient *lotus.LotusClient
	if cmdGoCar.ImportFlag {
		lotusClient, err = lotus.LotusGetClient(cmdGoCar.LotusClientApiUrl, cmdGoCar.LotusClientAccessToken)
		if err != nil {
			logs.GetLogger().Error(err)
			return nil, err
		}
	}

	fileDescs := []*libmodel.FileDesc{}
	for i, line := range lines {
		if i == 0 {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			err := fmt.Errorf("not enough fields in %s", manifestFilename)
			logs.GetLogger().Error(err)
			return nil, err
		}

		fileDesc := libmodel.FileDesc{}
		fileDesc.PayloadCid = fields[0]
		fileDesc.CarFileName = fileDesc.PayloadCid + ".car"
		fileDesc.CarFileUrl = fileDesc.CarFileName
		fileDesc.CarFilePath = filepath.Join(cmdGoCar.OutputDir, fileDesc.CarFileName)
		fileDesc.PieceCid = fields[2]
		fileDesc.CarFileSize = utils.GetInt64FromStr(fields[3])

		if cmdGoCar.ImportFlag {
			//pieceCid, err := lotusClient.LotusClientCalcCommP(fileDesc.CarFilePath)
			//if err != nil {
			//	logs.GetLogger().Error(err)
			//	return nil, err
			//}
			//
			//fileDesc.PieceCid = *pieceCid
			dataCid, err := lotusClient.LotusClientImport(fileDesc.CarFilePath, true)
			if err != nil {
				logs.GetLogger().Error(err)
				return nil, err
			}

			fileDesc.PayloadCid = *dataCid
		}

		carFileDetail := fields[4]
		for i := 5; i < len(fields); i++ {
			carFileDetail = carFileDetail + "," + fields[i]
		}

		manifestDetail := ManifestDetail{}
		err = json.Unmarshal([]byte(carFileDetail), &manifestDetail)
		if err != nil {
			logs.GetLogger().Error("Failed to parse: ", carFileDetail)
			return nil, err
		}

		if cmdGoCar.GocarFolderBased {
			fileDesc.SourceFileName = filepath.Base(cmdGoCar.InputDir)
			fileDesc.SourceFilePath = cmdGoCar.InputDir
			for _, link := range manifestDetail.Link {
				fileDesc.SourceFileSize = fileDesc.SourceFileSize + link.Size
			}
		} else {
			fileDesc.SourceFileName = manifestDetail.Link[0].Name
			fileDesc.SourceFilePath = filepath.Join(cmdGoCar.InputDir, fileDesc.SourceFileName)
			fileDesc.SourceFileSize = int64(manifestDetail.Link[0].Size)
		}

		if cmdGoCar.GenerateMd5 {
			if utils.IsFileExistsFullPath(fileDesc.SourceFilePath) {
				srcFileMd5, err := checksum.MD5sum(fileDesc.SourceFilePath)
				if err != nil {
					logs.GetLogger().Error(err)
					return nil, err
				}
				fileDesc.SourceFileMd5 = srcFileMd5
			}

			carFileMd5, err := checksum.MD5sum(fileDesc.CarFilePath)
			if err != nil {
				logs.GetLogger().Error(err)
				return nil, err
			}
			fileDesc.CarFileMd5 = carFileMd5
		}

		fileDescs = append(fileDescs, &fileDesc)
	}

	_, err = WriteCarFilesToFiles(fileDescs, cmdGoCar.OutputDir, JSON_FILE_NAME_CAR_UPLOAD, CSV_FILE_NAME_CAR_UPLOAD)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	return fileDescs, nil
}
