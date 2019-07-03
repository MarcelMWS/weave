package cron

import (
	"github.com/iov-one/weave/migration"
	"github.com/iov-one/weave/orm"
)

func init() {
	migration.MustRegister(1, &TaskResult{}, migration.NoModification)
}

var _ orm.CloneableData = (*TaskResult)(nil)

func (t *TaskResult) Validate() error {
	return nil
}

func (t *TaskResult) Copy() orm.CloneableData {
	return &TaskResult{
		Metadata:   t.Metadata.Copy(),
		Successful: t.Successful,
		Info:       t.Info,
	}
}

// NewTaskResultBucket returns a bucket for storing Task results.
func NewTaskResultBucket() orm.ModelBucket {
	b := orm.NewModelBucket("trs", &TaskResult{})
	return migration.NewModelBucket("cron", b)
}
