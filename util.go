package swim

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"

	"github.com/DE-labtory/iLogger"
)

type status = int32

const (
	AVAILABLE status = iota
	DIE
)

type Task func() (interface{}, error)

type TaskResponse struct {
	Payload interface{}
	Err     error
}

type TaskRunner struct {
	task     Task
	ctx      context.Context
	stopFlag int32
}

func NewTaskRunner(task Task, ctx context.Context) *TaskRunner {
	return &TaskRunner{
		task: task,
		ctx:  ctx,
	}
}

func (t *TaskRunner) stop() {
	atomic.CompareAndSwapInt32(&t.stopFlag, AVAILABLE, DIE)
}

func (t *TaskRunner) toDie() bool {
	return atomic.LoadInt32(&(t.stopFlag)) == DIE
}

func (t *TaskRunner) Start() TaskResponse {
	done := make(chan TaskResponse)
	defer func() {
		t.stop()
	}()

	go func() {
		result, err := t.task()
		if t.toDie() {
			return
		}

		if err != nil {
			iLogger.Errorf(nil, "[TaskRunner] error occured: [%s]", err.Error())
			done <- TaskResponse{
				Payload: nil,
				Err:     err,
			}
			return
		}
		done <- TaskResponse{
			Payload: result,
			Err:     nil,
		}
	}()

	select {
	case resp := <-done:
		return resp
	case <-t.ctx.Done():
		iLogger.Infof(nil, "[TaskRunner] receive cancel signal, quit")
		return TaskResponse{}
	}
}

func ParseHostPort(address string) (net.IP, uint16, error) {
	shost, sport, err := net.SplitHostPort(address)
	if err != nil {
		return net.IP{}, 0, err
	}

	host := net.ParseIP(shost)
	if net.IP(nil).Equal(host) {
		return net.IP{}, 0, errors.New("invalid ip format")
	}

	port, err := strconv.ParseUint(sport, 10, 16)
	if err != nil {
		return net.IP{}, 0, err
	}

	return host, uint16(port), nil
}
