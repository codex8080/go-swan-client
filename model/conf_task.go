package model

import (
	"path/filepath"
	"time"

	"go-swan-client/common/constants"
	"go-swan-client/common/utils"
	"go-swan-client/config"
)

type ConfTask struct {
	SwanApiUrl                 string
	SwanApiKey                 string
	SwanAccessToken            string
	PublicDeal                 bool
	BidMode                    int
	VerifiedDeal               bool
	OfflineMode                bool
	FastRetrieval              bool
	MaxPrice                   string
	StorageServerType          string
	WebServerDownloadUrlPrefix string
	ExpireDays                 int
	OutputDir                  string
	InputDir                   string
	TaskName                   *string
	MinerFid                   *string
	Dataset                    *string
	Description                *string
	StartEpoch                 int
	StartEpochIntervalHours    int
}

func GetConfTask(inputDir string, outputDir *string, taskName, minerFid, dataset, description *string) *ConfTask {
	startEpochIntervalHours := config.GetConfig().Sender.StartEpochHours
	startEpoch := utils.GetCurrentEpoch() + (startEpochIntervalHours+1)*constants.EPOCH_PER_HOUR

	confTask := &ConfTask{
		SwanApiUrl:                 config.GetConfig().Main.SwanApiUrl,
		SwanApiKey:                 config.GetConfig().Main.SwanApiKey,
		SwanAccessToken:            config.GetConfig().Main.SwanAccessToken,
		PublicDeal:                 config.GetConfig().Sender.PublicDeal,
		BidMode:                    config.GetConfig().Sender.BidMode,
		VerifiedDeal:               config.GetConfig().Sender.VerifiedDeal,
		OfflineMode:                config.GetConfig().Sender.OfflineMode,
		FastRetrieval:              config.GetConfig().Sender.FastRetrieval,
		MaxPrice:                   config.GetConfig().Sender.MaxPrice,
		StorageServerType:          config.GetConfig().Main.StorageServerType,
		WebServerDownloadUrlPrefix: config.GetConfig().WebServer.DownloadUrlPrefix,
		ExpireDays:                 config.GetConfig().Sender.ExpireDays,
		OutputDir:                  filepath.Join(config.GetConfig().Sender.OutputDir, time.Now().Format("2006-01-02_15:04:05")),
		InputDir:                   inputDir,
		TaskName:                   taskName,
		MinerFid:                   minerFid,
		Dataset:                    dataset,
		Description:                description,
		StartEpochIntervalHours:    startEpochIntervalHours,
		StartEpoch:                 startEpoch,
	}

	if outputDir != nil && len(*outputDir) != 0 {
		confTask.OutputDir = *outputDir
	}

	return confTask
}
