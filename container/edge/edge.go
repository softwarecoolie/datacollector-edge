package edge

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/streamsets/datacollector-edge/container/common"
	"github.com/streamsets/datacollector-edge/container/dpm"
	"github.com/streamsets/datacollector-edge/container/execution/manager"
	"github.com/streamsets/datacollector-edge/container/http"
	"github.com/streamsets/datacollector-edge/container/store"
	"github.com/streamsets/datacollector-edge/container/util"
	"log"
	"os"
	"path"
	"strings"
)

const (
	DefaultLogFilePath    = "/log/edge.log"
	DefaultConfigFilePath = "/etc/edge.conf"
	DEBUG                 = "DEBUG"
	WARN                  = "WARN"
	ERROR                 = "ERROR"
	INFO                  = "INFO"
)

type DataCollectorEdgeMain struct {
	config        *Config
	buildInfo     *common.BuildInfo
	runtimeInfo   *common.RuntimeInfo
	webServerTask *http.WebServerTask
	manager       *manager.PipelineManager
}

func DoMain() {
	debugFlag := flag.Bool("debug", false, "Debug flag")
	startFlag := flag.String("start", "", "Start Pipeline flag")
	runtimeParametersFlag := flag.String("runtimeParameters", "", "Runtime Parameters flag")
	flag.Parse()

	dataExtractor, _ := newDataCollectorEdge(*debugFlag)

	if len(*startFlag) > 0 {
		var runtimeParameters map[string]interface{}
		if len(*runtimeParametersFlag) > 0 {
			err := json.Unmarshal([]byte(*runtimeParametersFlag), &runtimeParameters)
			if err != nil {
				panic(err)
			}
		}

		fmt.Println("Starting Pipeline: ", *startFlag)
		state, err := dataExtractor.manager.GetRunner(*startFlag).GetStatus()
		if state != nil && state.Status == common.RUNNING {
			// If status is running, change it back to stopped
			dataExtractor.manager.StopPipeline(*startFlag)
		}

		state, err = dataExtractor.manager.StartPipeline(*startFlag, runtimeParameters)
		if err != nil {
			panic(err)
		}
		stateJson, _ := json.Marshal(state)
		fmt.Println(string(stateJson))
	}

	dataExtractor.webServerTask.Run()
}

func newDataCollectorEdge(debugFlag bool) (*DataCollectorEdgeMain, error) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	baseDir := strings.TrimSuffix(path.Dir(ex), "/bin")
	initializeLog(debugFlag, baseDir)

	log.Println("[INFO] Base Dir: ", baseDir)
	fmt.Println("Base Dir: ", baseDir)

	config := NewConfig()
	config.FromTomlFile(baseDir + DefaultConfigFilePath)

	hostName, _ := os.Hostname()
	var httpUrl = "http://" + hostName + config.Http.BindAddress

	buildInfo, _ := common.NewBuildInfo()
	runtimeInfo, _ := common.NewRuntimeInfo(httpUrl, baseDir)
	pipelineStoreTask := store.NewFilePipelineStoreTask(*runtimeInfo)
	pipelineManager, _ := manager.NewManager(config.Execution, *runtimeInfo, pipelineStoreTask)
	webServerTask, _ := http.NewWebServerTask(config.Http, buildInfo, pipelineManager, pipelineStoreTask)
	dpm.RegisterWithDPM(config.DPM, buildInfo, runtimeInfo)

	return &DataCollectorEdgeMain{
		config:        config,
		buildInfo:     buildInfo,
		runtimeInfo:   runtimeInfo,
		webServerTask: webServerTask,
		manager:       pipelineManager,
	}, nil
}

func initializeLog(debugFlag bool, baseDir string) {
	minLevel := util.LogLevel(WARN)
	if debugFlag {
		minLevel = util.LogLevel(DEBUG)
	}

	loggerFile, _ := os.OpenFile(baseDir+DefaultLogFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	logFilter := &util.LevelFilter{
		Levels:   []util.LogLevel{DEBUG, WARN, ERROR, INFO},
		MinLevel: minLevel,
		Writer:   loggerFile,
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(logFilter)
}
