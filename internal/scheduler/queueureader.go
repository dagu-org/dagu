// TO DO - implement a scheduler which
/*
- checks number of dags running at the same time
- if numberOfRunningDags < queueLength(from config)
- it will periodically checks from the queue.json and dequeue first params if
- this will happen until queue.json is `[]`
*/

package scheduler

import (
	"log"
	"time"

	// "path/filepath"

	"errors"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/persistence/model"

	// "github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/stats"
)

var (
	errQueueEmpty = errors.New("queue empty")
)

type newQueueReaderArgs struct {
	QueueDir  string
	Logger    logger.Logger
	DataStore persistence.DataStores
}

type queueReaderImpl struct {
	queueDir   string
	logger     logger.Logger
	datastore  persistence.DataStores
	queueStore persistence.QueueStore
}

type StartOptions struct {
	Params           string
	Quiet            bool
	FromWaitingQueue bool
}

func newQueueReader(args newQueueReaderArgs) *queueReaderImpl {
	// fmt.Print("queue:", queueDir)
	qr := &queueReaderImpl{
		queueDir:  args.QueueDir,
		logger:    args.Logger,
		datastore: args.DataStore,
	}

	// log.Print(qr)

	if err := qr.initQueue(); err != nil {
		qr.logger.Error("failed to init queue", err)
	}

	return qr
}

func (qr *queueReaderImpl) Start(done chan any) {
	go qr.watchQueue(done)
}

func (qr *queueReaderImpl) watchQueue(done chan any) {
	log.Print("queue being watched")
	const checkInterval = 2 * time.Second // Check interval in seconds
	errs := make(chan error)
	ticker := time.NewTicker(checkInterval)
	// cfg, _ := config.Load()

	for {
		select {
		case <-ticker.C:
			runFi, err := qr.ReadFileQueue()
			if err != nil {
				errs <- err
				return
			}
			if len(runFi) != 0 {
				for i := 0; i < len(runFi); i++ {
					log.Print("dags readFileQueue:", runFi[i])
					go qr.execute(runFi[i])
				}
			}
			if runFi == nil {
				continue
			}
		case <-done:
			return
		}
	}
}

// edge case - where noOfDags in queue is less then queueLength
func (qr *queueReaderImpl) ReadFileQueue() ([]*model.Queue, error) {
	var params []*model.Queue
	cfg, _ := config.Load()
	stats := stats.NewStatsStore(cfg.StatsDir)
	queueLength := cfg.DAGQueueLength
	noOfRunningDAGS, _ := stats.GetRunningDags()
	if queueLength > noOfRunningDAGS {
		for i := 0; i < queueLength-noOfRunningDAGS; i++ {
			DAGparam, err := qr.queueStore.Dequeue()
			// if the queue is empty
			if DAGparam == nil {
				return params, nil
			}
			// if there is any error reading queue.json
			if err != nil {
				qr.logger.Error("error reading queue", "error", err)
				return nil, nil
			}
			log.Print("para", DAGparam)
			params = append(params, DAGparam)
		}
		return params, nil
	} else {
		return nil, nil
	}
}

func (qr *queueReaderImpl) execute(dagFile *model.Queue) {
	log.Print("executing", dagFile.Name)
	cfg, _ := config.Load()
	logger := logger.NewLogger(logger.NewLoggerArgs{
		Debug:  cfg.Debug,
		Format: cfg.LogFormat,
	})
	cli := client.New(qr.datastore, cfg.Executable, cfg.WorkDir, logger)

	dag, _ := cli.GetStatus(dagFile.Name)
	if err := cli.Start(dag.DAG, client.StartOptions{
		Quiet:            false,
		FromWaitingQueue: true,
	}); err != nil {
		qr.logger.Error("error starting the dag from queue:", "error", err)
	}
	defer log.Print("executing from queue", dagFile.Name)
}

func (qr *queueReaderImpl) initQueue() error {
	// TODO: do not use the persistence package directly.
	qr.queueStore = qr.datastore.QueueStore()
	err := qr.queueStore.Create()
	if err != nil {
		return err
	}
	return nil
}
