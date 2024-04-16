package gcron

import (
	"fmt"
	"testing"
	"time"

	"github.com/ijkzen/gcron"
)

func TestStart(t *testing.T) {
	jobPool := gcron.NewJobPool()
	jobPool.Add(getTestJob())
	jobPool.Start()
	time.Sleep(10 * time.Second)
}

func TestAdd(t *testing.T) {
	jobPool := gcron.NewJobPool()
	jobPool.Start()
	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		jobPool.Add(getTestJob())
	}()
	time.Sleep(10 * time.Second)
	fmt.Printf("\nend: %s\n", time.Now())
}

func TestRemove(t *testing.T) {
	jobPool := gcron.NewJobPool()
	jobPool.Add(getTestJob())
	jobPool.Start()
	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		jobPool.Remove(getTestJob())
	}()
	time.Sleep(10 * time.Second)
}

func TestUpdate(t *testing.T) {
	jobPool := gcron.NewJobPool()
	jobPool.Add(getTestJob())
	jobPool.Start()
	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		jobPool.Update(getUpdateTestJob())
	}()
	time.Sleep(10 * time.Second)
}

func TestStop(t *testing.T) {
	jobPool := gcron.NewJobPool()
	jobPool.Add(getTestJob())
	jobPool.Start()
	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		jobPool.Stop()
	}()
	time.Sleep(10 * time.Second)
}

func TestRestore(t *testing.T) {
	jobPool := gcron.NewJobPool()
	jobPool.Init(func(jobMap map[string]gcron.WrapFunction) {
		jobMap["test"] = func() {
			println(time.Now().Second())
		}
	})
	jobPool.Start()

	time.Sleep(10 * time.Second)
}

func getTestJob() *gcron.Job {
	return &gcron.Job{
		Key:         "test",
		Name:        "print seconds",
		Description: "print seconds",
		Active:      true,
		Function: func() {
			println(time.Now().Second())
		},
		Schedule:           nil,
		ScheduleExpression: "@every 1s",
	}
}

func getUpdateTestJob() *gcron.Job {
	return &gcron.Job{
		Key:         "test",
		Name:        "print seconds",
		Description: "print seconds",
		Active:      true,
		Function: func() {
			println(time.Now().Second())
		},
		Schedule:           nil,
		ScheduleExpression: "@every 2s",
	}
}
