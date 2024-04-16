package gcron

import (
	"encoding/json"
	"os"
	"sync"
)

type Persistence interface {
	Restore() []*Job
	Save(jobs []*Job) error
}

type DefaultPersistence struct {
	mutex sync.Mutex
}

var defaultCronFile = "cron.json"

func (p *DefaultPersistence) Restore() []*Job {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	content, err := os.ReadFile(defaultCronFile)
	if err != nil {
		return make([]*Job, 0)
	}

	var jobs []*Job
	if err := json.Unmarshal(content, &jobs); err != nil {
		return make([]*Job, 0)
	}

	return jobs
}

func (p *DefaultPersistence) Save(jobs []*Job) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	content, err := json.Marshal(jobs)
	if err != nil {
		return err
	}

	if err := os.WriteFile(defaultCronFile, content, 0644); err != nil {
		return err
	}

	return nil
}
