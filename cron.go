package gcron

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"
)

type Job struct {
	Key                string       `json:"key"`
	Name               string       `json:"name"`
	Description        string       `json:"description"`
	Active             bool         `json:"active"`
	Function           WrapFunction `json:"-"`
	FunctionName       string       `json:"functionName"`
	Schedule           Schedule     `json:"-"`
	ScheduleExpression string       `json:"scheduleExpression"`
	PreviousTime       time.Time    `json:"previousTime"`
	NextTime           time.Time    `json:"nextTime"`
	Running            bool         `json:"running"`
	ModifiedTime       time.Time    `json:"modifiedTime"`
}

func (j *Job) Merge(newJob *Job) {
	j.Name = newJob.Name
	j.Description = newJob.Description
	j.Active = newJob.Active
	j.ScheduleExpression = newJob.ScheduleExpression
}

type WrapFunction func()

type JobPool struct {
	jobs               []*Job
	jobMap             map[string]WrapFunction
	stopChan           chan struct{}
	addChan            chan *Job
	removeChan         chan *Job
	updateChan         chan *Job
	running            bool
	mutex              sync.Mutex
	wg                 sync.WaitGroup
	location           *time.Location
	customPersistence  Persistence
	defaultPersistence Persistence
	scheduleParser     ScheduleParser
}

func NewJobPool() *JobPool {
	return &JobPool{
		jobs:               make([]*Job, 0),
		jobMap:             make(map[string]WrapFunction),
		stopChan:           make(chan struct{}),
		addChan:            make(chan *Job),
		removeChan:         make(chan *Job),
		updateChan:         make(chan *Job),
		running:            false,
		mutex:              sync.Mutex{},
		wg:                 sync.WaitGroup{},
		location:           time.Local,
		customPersistence:  nil,
		defaultPersistence: &DefaultPersistence{},
		scheduleParser:     standardParser,
	}
}

func (p *JobPool) Init(fn func(jobMap map[string]WrapFunction)) {
	localJobs := p.restore()
	fn(p.jobMap)
	for _, job := range localJobs {
		if function, ok := p.jobMap[job.Key]; ok {
			job.Function = function
			job.FunctionName = getFuncName(function)
			schedule, err := p.scheduleParser.Parse(job.ScheduleExpression)
			if err != nil {
				continue
			}
			job.Schedule = schedule
			job.Running = false
			p.jobs = append(p.jobs, job)
		}
	}
}

func (p *JobPool) restore() []*Job {
	if p.customPersistence != nil {
		return p.customPersistence.Restore()
	}

	return p.defaultPersistence.Restore()
}

func (p *JobPool) save() error {
	if p.customPersistence != nil {
		return p.customPersistence.Save(p.jobs)
	}

	return p.defaultPersistence.Save(p.jobs)
}

func (p *JobPool) Add(job *Job) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		p.addChan <- job
		return nil
	} else {
		return p.add(job)
	}
}

func (p *JobPool) jobIndex(job *Job) int {
	for i, j := range p.jobs {
		if j.Key == job.Key {
			return i
		}
	}

	return -1
}

func (p *JobPool) add(job *Job) error {
	if index := p.jobIndex(job); index != -1 {
		return fmt.Errorf("job %s already exists", job.Key)
	}
	job.Schedule, _ = p.scheduleParser.Parse(job.ScheduleExpression)
	job.NextTime = job.Schedule.Next(p.now())
	job.ModifiedTime = p.now()
	job.FunctionName = getFuncName(job.Function)
	p.jobs = append(p.jobs, job)
	p.save()
	return nil
}

func (p *JobPool) Remove(job *Job) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		p.removeChan <- job
	} else {
		p.remove(job)
	}
}

func (p *JobPool) remove(job *Job) {
	jobIndex := p.jobIndex(job)
	if jobIndex == -1 {
		return
	}
	p.jobs = append(p.jobs[:jobIndex], p.jobs[jobIndex+1:]...)
	p.save()
}

func (p *JobPool) Update(job *Job) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		p.updateChan <- job
	} else {
		p.update(job)
	}
}

func (p *JobPool) update(job *Job) {
	for _, j := range p.jobs {
		if j.Key == job.Key {
			j.Merge(job)
			var err error
			j.Schedule, err = p.scheduleParser.Parse(j.ScheduleExpression)
			if err != nil {
				fmt.Printf("error parsing schedule: %v\n", err)
				return
			}
			j.NextTime = j.Schedule.Next(p.now())
			j.ModifiedTime = time.Now().In(p.location)
		}
	}
	p.save()
}

func getFuncName(fn WrapFunction) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

func (p *JobPool) Start() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.running {
		return
	}

	p.running = true
	go p.run()
}

func (p *JobPool) run() {
	for _, job := range p.jobs {
		job.NextTime = job.Schedule.Next(p.now())
	}

	for {
		sort.Slice(p.jobs, func(i, j int) bool {
			return p.jobs[i].NextTime.Before(p.jobs[j].NextTime)
		})

		var timer *time.Timer
		if len(p.jobs) == 0 || p.jobs[0].NextTime.IsZero() {
			timer = time.NewTimer(100000 * time.Hour)
		} else {
			duration := p.jobs[0].NextTime.Sub(p.now())
			timer = time.NewTimer(duration)
		}

		for {
			select {
			case now := <-timer.C:
				now = now.In(p.location)
				for _, job := range p.jobs {
					if job.NextTime.After(now) || job.NextTime.IsZero() {
						break
					}
					job.PreviousTime = job.NextTime
					job.NextTime = job.Schedule.Next(now)
					if job.Active {
						go p.startJob(job)
					}
				}

			case newJob := <-p.addChan:
				timer.Stop()
				p.add(newJob)

			case removeJob := <-p.removeChan:
				timer.Stop()
				p.remove(removeJob)

			case updateJob := <-p.updateChan:
				timer.Stop()
				p.update(updateJob)

			case <-p.stopChan:
				timer.Stop()
				return
			}
			break
		}
	}
}

func (p *JobPool) now() time.Time {
	return time.Now().In(p.location)
}

func (p *JobPool) startJob(job *Job) {
	p.wg.Add(1)
	job.Running = true
	job.Function()
	p.wg.Done()
	job.Running = false
}

func (p *JobPool) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.running {
		p.stopChan <- struct{}{}
		p.running = false
	}

	p.wg.Wait()
}

func (p *JobPool) GetJobList() []*Job {
	return p.jobs
}

func (p *JobPool) SetCustomPersistence(persistence Persistence) {
	p.customPersistence = persistence
}
