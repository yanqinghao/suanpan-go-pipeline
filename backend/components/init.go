package components

import (
	"goPipeline/utils"
	"strings"
	"sync"

	socketio "github.com/googollee/go-socket.io"
	"github.com/xuelang-group/suanpan-go-sdk/suanpan/v1/log"
)

type NodeAction interface {
	Run(inputData RequestData, wg *sync.WaitGroup, stopChan chan bool)
	UpdateInput(inputData RequestData, wg *sync.WaitGroup, stopChan chan bool)
	Main(inputData RequestData) (map[string]interface{}, error)
}

type Node struct {
	TriggeredPorts []string
	PreviousNodes  []*Node
	NextNodes      []*Node
	InputData      map[string]interface{}
	OutputData     map[string]interface{}
	PortConnects   map[string][]string
	Config         map[string]interface{}
	Id             string
	Key            string
	Run            func(currentNode Node, inputData RequestData, wg *sync.WaitGroup, stopChan chan bool, server *socketio.Server)
	// dumpOutput    func(currentNode Node, outputData map[string]interface{})
	UpdateInput func(currentNode Node, inputData RequestData, wg *sync.WaitGroup, stopChan chan bool)
	loadInput   func(currentNode Node, inputData RequestData) error
	main        func(currentNode Node, inputData RequestData) (map[string]interface{}, error)
	initNode    func(currentNode Node) error
	Status      int // 0: stoped 1： running -1：error
}

type RequestData struct {
	Data  string
	ID    string
	Extra string
}

func (c *Node) Init(nodeType string) {
	c.Run = Run
	c.UpdateInput = UpdateInput
	// c.dumpOutput = dumpOutput
	switch nodeType {
	case "StreamIn":
		c.main = streamInMain
		c.loadInput = streamInLoadInput
	case "StreamOut":
		c.main = streamOutMain
	case "JsonExtractor":
		c.main = jsonExtractorMain
	case "DataSync":
		c.main = dataSyncMain
	case "ExecutePythonScript":
		c.main = pyScriptMain
	case "PostgresReader":
		c.main = postgresReaderMain
		c.initNode = postgresInit
	case "PostgresSqlExecuter":
		c.main = postgresExecutorMain
		c.initNode = postgresInit
	case "PostgresWriter":
		c.main = postgresWriterMain
		c.initNode = postgresInit
	default:
	}
}

func Run(currentNode Node, inputData RequestData, wg *sync.WaitGroup, stopChan chan bool, server *socketio.Server) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("节点%s(%s)运行异常，错误日志：%s", currentNode.Key, currentNode.Id, err)
		}
		wg.Done()
	}()
	select {
	case <-stopChan:
		log.Infof("节点%s(%s)运行被中断", currentNode.Key, currentNode.Id)
	default:
		receiveInputs := false
		for _, v := range currentNode.InputData {
			if v != nil {
				receiveInputs = true
			}
		}
		if len(inputData.Data) > 0 || receiveInputs {
			currentNode.Status = 1
			outputData, err := currentNode.main(currentNode, inputData)
			if err != nil {
				log.Infof("Error occur when running node: %s, error info: %s", currentNode.Key, err.Error())
				currentNode.Status = -1
				if server != nil {
					server.BroadcastToNamespace("/", "notify.process.status", map[string]int{currentNode.Id: -1})
					server.BroadcastToNamespace("/", "notify.process.error", map[string]string{currentNode.Id: err.Error()})
				}
			} else {
				log.Infof("节点%s(%s)运行成功", currentNode.Key, currentNode.Id)
				readyToRun := make([]string, 0)
				triggeredPorts := make(map[string][]string)
				for port, data := range outputData { //map[out1:true]
					for _, tgt := range currentNode.PortConnects[port] {
						tgtInfo := strings.Split(tgt, "-")
						for i := range currentNode.NextNodes {
							if currentNode.NextNodes[i].Id == tgtInfo[0] {
								log.Infof("数据下发到节点%s(%s)", currentNode.NextNodes[i].Key, currentNode.NextNodes[i].Id)
								currentNode.NextNodes[i].InputData[tgtInfo[1]] = data
								triggeredPorts[currentNode.NextNodes[i].Id] = append(triggeredPorts[currentNode.NextNodes[i].Id], tgtInfo[1])
								if !utils.SlicesContain(readyToRun, currentNode.NextNodes[i].Id) {
									readyToRun = append(readyToRun, currentNode.NextNodes[i].Id)
								}
							}
						}
					}
				}
				for i := range currentNode.NextNodes {
					if utils.SlicesContain(readyToRun, currentNode.NextNodes[i].Id) {
						currentNode.NextNodes[i].TriggeredPorts = triggeredPorts[currentNode.NextNodes[i].Id]
						wg.Add(1)
						go currentNode.NextNodes[i].Run(*currentNode.NextNodes[i], RequestData{ID: inputData.ID, Extra: inputData.Extra}, wg, stopChan, server)
					}
				}
				currentNode.Status = 0
				if server != nil {
					server.BroadcastToNamespace("/", "notify.process.status", map[string]int{currentNode.Id: 0})
				}
			}
		}
	}
}

func UpdateInput(currentNode Node, inputData RequestData, wg *sync.WaitGroup, stopChan chan bool) {
	defer wg.Done()
	select {
	case <-stopChan:
		log.Info("Recive stop event")
	default:
		err := currentNode.loadInput(currentNode, inputData)
		if err != nil {
			log.Infof("Error occur when running node: %s, error info: %s", currentNode.Key, err.Error())
		}
	}
}

// func dumpOutput(currentNode Node, outputData map[string]interface{}) {
// 	for port, data := range outputData { //map[out1:true]
// 		for _, tgt := range currentNode.PortConnects[port] {
// 			tgtInfo := strings.Split(tgt, "-")
// 			for _, node := range currentNode.NextNodes {
// 				if node.Id == tgtInfo[0] {
// 					node.InputData[tgtInfo[1]] = data
// 				}
// 			}
// 		}
// 	}
// }
