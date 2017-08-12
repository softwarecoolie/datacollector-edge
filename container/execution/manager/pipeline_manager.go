package manager

import (
	"github.com/streamsets/datacollector-edge/container/common"
	"github.com/streamsets/datacollector-edge/container/execution"
	"github.com/streamsets/datacollector-edge/container/execution/runner"
	"github.com/streamsets/datacollector-edge/container/store"
)

type PipelineManager struct {
	config            execution.Config
	runnerMap         map[string]*runner.StandaloneRunner
	runtimeInfo       common.RuntimeInfo
	pipelineStoreTask store.PipelineStoreTask
}

func (p *PipelineManager) GetRunner(pipelineId string) *runner.StandaloneRunner {
	if p.runnerMap[pipelineId] == nil {
		pRunner, err := runner.NewStandaloneRunner(pipelineId, p.config, p.runtimeInfo, p.pipelineStoreTask)
		if err != nil {
			panic(err)
		}
		p.runnerMap[pipelineId] = pRunner
	}
	return p.runnerMap[pipelineId]
}

func (p *PipelineManager) StartPipeline(
	pipelineId string,
	runtimeParameters map[string]interface{},
) (*common.PipelineState, error) {
	return p.GetRunner(pipelineId).StartPipeline(runtimeParameters)
}

func (p *PipelineManager) StopPipeline(pipelineId string) (*common.PipelineState, error) {
	return p.GetRunner(pipelineId).StopPipeline()
}

func (p *PipelineManager) ResetOffset(pipelineId string) error {
	return p.GetRunner(pipelineId).ResetOffset()
}

func NewManager(
	config execution.Config,
	runtimeInfo common.RuntimeInfo,
	pipelineStoreTask store.PipelineStoreTask,
) (*PipelineManager, error) {
	pipelineManager := PipelineManager{
		config:            config,
		runnerMap:         make(map[string]*runner.StandaloneRunner),
		runtimeInfo:       runtimeInfo,
		pipelineStoreTask: pipelineStoreTask,
	}

	return &pipelineManager, nil
}
